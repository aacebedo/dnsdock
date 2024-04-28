/* docker.go
 *
 * Copyright (C) 2016 Alexandre ACEBEDO
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file for details.
 */

package core

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aacebedo/dnsdock/internal/servers"
	"github.com/aacebedo/dnsdock/internal/utils"
	"github.com/cenkalti/backoff/v4"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// DockerProvider is the name of the provider used for services added by the Docker client
const DockerProvider = "docker"

// DockerManager is the entrypoint to the docker daemon
type DockerManager struct {
	config *utils.Config
	list   servers.ServiceListProvider
	client *client.Client
	cancel context.CancelFunc
}

// NewDockerManager creates a new DockerManager
func NewDockerManager(c *utils.Config, list servers.ServiceListProvider, tlsConfig *tls.Config) (*DockerManager, error) {
	dclient, err := client.NewClientWithOpts(client.WithHost(c.DockerHost), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &DockerManager{config: c, list: list, client: dclient}, nil
}

// Start starts the DockerManager
func (d *DockerManager) Start() (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	go func() {
		perr := backoff.RetryNotify(func() error {
			return d.run(ctx)
		}, backoff.WithContext(backoff.NewExponentialBackOff(), ctx), func(err error, d time.Duration) {
			logger.Errorf("Error running docker manager, retrying in %v: %s", d, err)
		})
		if perr != nil {
			logger.Errorf("Unrecoverable error running docker manager: %s", perr)
			cancel()
		}
	}()

	return nil
}

func (d *DockerManager) run(ctx context.Context) error {
	messageChan, errorChan := d.client.Events(ctx, types.EventsOptions{
		Filters: filters.NewArgs(filters.Arg("type", "container")),
	})

	containers, err := d.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting containers: %w", err)
	}

	services := make(map[string]struct{})
	for _, container := range containers {
		service, err := d.getService(container.ID)
		if err != nil {
			return fmt.Errorf("error getting service: %w", err)
		}
		err = d.list.AddService(container.ID, *service)
		if err != nil {
			return fmt.Errorf("error adding service: %w", err)
		}
		services[container.ID] = struct{}{}
	}

	for id, srv := range d.list.GetAllServices() {
		if _, ok := services[id]; !ok && srv.Provider == DockerProvider {
			err := d.list.RemoveService(id)
			if err != nil {
				return fmt.Errorf("error removing service: %w", err)
			}
		}
	}

	for {
		select {
		case m := <-messageChan:
			err := d.handler(m)
			if err != nil {
				return err
			}
		case err := <-errorChan:
			return err
		case <-ctx.Done():
			return nil
		}
	}
}

func (d *DockerManager) handler(m events.Message) error {
	switch m.Action {
	case "create":
		return d.createHandler(m)
	case "start":
		return d.startHandler(m)
	case "unpause":
		return d.startHandler(m)
	case "die":
		return d.stopHandler(m)
	case "pause":
		return d.stopHandler(m)
	case "destroy":
		return d.destroyHandler(m)
	case "rename":
		return d.renameHandler(m)
	}
	return nil
}

func (d *DockerManager) createHandler(m events.Message) error {
	logger.Debugf("Created container '%s'", m.ID)
	if d.config.All {
		service, err := d.getService(m.ID)
		if err != nil {
			return fmt.Errorf("error getting service: %w", err)
		}
		err = d.list.AddService(m.ID, *service)
		if err != nil {
			return fmt.Errorf("error adding service: %w", err)
		}
	}
	return nil
}

func (d *DockerManager) startHandler(m events.Message) error {
	logger.Debugf("Started container '%s'", m.ID)
	if !d.config.All {
		service, err := d.getService(m.ID)
		if err != nil {
			return fmt.Errorf("error getting service: %w", err)
		}
		err = d.list.AddService(m.ID, *service)
		if err != nil {
			return fmt.Errorf("error adding service: %w", err)
		}
	}
	return nil
}

func (d *DockerManager) stopHandler(m events.Message) error {
	logger.Debugf("Stopped container '%s'", m.ID)
	if !d.config.All {
		err := d.list.RemoveService(m.ID)
		if err != nil {
			return fmt.Errorf("error removing service: %w", err)
		}
	} else {
		logger.Debugf("Stopped container '%s' not removed as --all argument is true", m.ID)
	}
	return nil
}

func (d *DockerManager) renameHandler(m events.Message) error {
	logger.Debugf("Renamed container '%s'", m.ID)
	err := d.list.RemoveService(m.ID)
	if err != nil {
		return fmt.Errorf("error removing service: %w", err)
	}
	service, err := d.getService(m.ID)
	if err != nil {
		return fmt.Errorf("error getting service: %w", err)
	}
	res := d.list.AddService(m.ID, *service)
	if res != nil {
		return fmt.Errorf("error removing service: %w", err)
	}
	return nil
}

func (d *DockerManager) destroyHandler(m events.Message) error {
	logger.Debugf("Destroy container '%s'", m.ID)
	if d.config.All {
		err := d.list.RemoveService(m.ID)
		if err != nil {
			return fmt.Errorf("error removing service: %w", err)
		}
	}
	return nil
}

// Stop stops the DockerManager
func (d *DockerManager) Stop() {
	d.cancel()
}

func (d *DockerManager) getService(id string) (*servers.Service, error) {
	desc, err := d.client.ContainerInspect(context.Background(), id)
	if err != nil {
		return nil, err
	}

	service := servers.NewService(DockerProvider)
	service.Aliases = make([]string, 0)

	service.Image = getImageName(desc.Config.Image)
	if imageNameIsSHA(service.Image, desc.Image) {
		logger.Warningf("Warning: Can't route %s, image %s is not a tag.", id[:10], service.Image)
		service.Image = ""
	}
	service.Name = cleanContainerName(desc.Name)

	switch len(desc.NetworkSettings.Networks) {
	case 0:
		logger.Warningf("Warning, no IP address found for container '%s' ", desc.Name)
	default:
		for _, value := range desc.NetworkSettings.Networks {
			ip := net.ParseIP(value.IPAddress)
			if ip != nil {
				service.IPs = append(service.IPs, ip)
			}
		}
	}

	service = overrideFromLabels(service, desc.Config.Labels)
	service = overrideFromEnv(service, splitEnv(desc.Config.Env))
	if service == nil {
		return nil, errors.New("Skipping " + id)
	}

	if d.config.CreateAlias {
		service.Aliases = append(service.Aliases, service.Name)
	}
	return service, nil
}

func getImageName(tag string) string {
	if index := strings.LastIndex(tag, "/"); index != -1 {
		tag = tag[index+1:]
	}
	if index := strings.LastIndex(tag, ":"); index != -1 {
		tag = tag[:index]
	}
	return tag
}

func imageNameIsSHA(image, sha string) bool {
	// Hard to make a judgement on small image names.
	if len(image) < 4 {
		return false
	}
	// Image name is not HEX
	matched, _ := regexp.MatchString("^[0-9a-f]+$", image)
	if !matched {
		return false
	}
	return strings.HasPrefix(sha, image)
}

func cleanContainerName(name string) string {
	return strings.Replace(name, "/", "", -1)
}

func splitEnv(in []string) (out map[string]string) {
	out = make(map[string]string, len(in))
	for _, exp := range in {
		parts := strings.SplitN(exp, "=", 2)
		var value string
		if len(parts) > 1 {
			value = strings.Trim(parts[1], " ") // trim just in case
		}
		out[strings.Trim(parts[0], " ")] = value
	}
	return
}

func overrideFromLabels(in *servers.Service, labels map[string]string) (out *servers.Service) {
	var region string
	for k, v := range labels {
		if k == "com.dnsdock.ignore" {
			return nil
		}

		if k == "com.dnsdock.alias" {
			in.Aliases = strings.Split(v, ",")
		}

		if k == "com.dnsdock.name" {
			in.Name = v
		}

		if k == "com.dnsdock.tags" {
			if len(v) == 0 {
				in.Name = ""
			} else {
				in.Name = strings.Split(v, ",")[0]
			}
		}

		if k == "com.dnsdock.image" {
			in.Image = v
		}

		if k == "com.dnsdock.ttl" {
			if ttl, err := strconv.Atoi(v); err == nil {
				in.TTL = ttl
			}
		}

		if k == "com.dnsdock.region" {
			region = v
		}

		if k == "com.dnsdock.ip_addr" {
			ipAddr := net.ParseIP(v)
			if ipAddr != nil {
				in.IPs = in.IPs[:0]
				in.IPs = append(in.IPs, ipAddr)
			}
		}

		if k == "com.dnsdock.prefix" {
			addrs := make([]net.IP, 0)
			for _, value := range in.IPs {
				if strings.HasPrefix(value.String(), v) {
					addrs = append(addrs, value)
				}
			}
			if len(addrs) == 0 {
				logger.Warningf("The prefix '%s' didn't match any IP addresses of service '%s', the service will be ignored", v, in.Name)
			}
			in.IPs = addrs
		}
	}

	if len(region) > 0 {
		in.Image = in.Image + "." + region
	}
	out = in
	return
}

func overrideFromEnv(in *servers.Service, env map[string]string) (out *servers.Service) {
	var region string
	for k, v := range env {
		if k == "DNSDOCK_IGNORE" || k == "SERVICE_IGNORE" {
			return nil
		}

		if k == "DNSDOCK_ALIAS" {
			in.Aliases = strings.Split(v, ",")
		}

		if k == "DNSDOCK_NAME" {
			in.Name = v
		}

		if k == "SERVICE_TAGS" {
			if len(v) == 0 {
				in.Name = ""
			} else {
				in.Name = strings.Split(v, ",")[0]
			}
		}

		if k == "DNSDOCK_IMAGE" || k == "SERVICE_NAME" {
			in.Image = v
		}

		if k == "DNSDOCK_TTL" {
			if ttl, err := strconv.Atoi(v); err == nil {
				in.TTL = ttl
			}
		}

		if k == "SERVICE_REGION" {
			region = v
		}

		if k == "DNSDOCK_IPADDRESS" {
			ipAddr := net.ParseIP(v)
			if ipAddr != nil {
				in.IPs = in.IPs[:0]
				in.IPs = append(in.IPs, ipAddr)
			}
		}

		if k == "DNSDOCK_PREFIX" {
			addrs := make([]net.IP, 0)
			for _, value := range in.IPs {
				if strings.HasPrefix(value.String(), v) {
					addrs = append(addrs, value)
				}
			}
			if len(addrs) == 0 {
				logger.Warningf("The prefix '%s' didn't match any IP address of  service '%s', the service will be ignored", v, in.Name)
			}
			in.IPs = addrs
		}
	}

	if len(region) > 0 {
		in.Image = in.Image + "." + region
	}
	out = in
	return
}
