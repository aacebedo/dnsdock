/* docker.go
 *
 * Copyright (C) 2016 Alexandre ACEBEDO
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file for details.
 */

package core

import (
	"crypto/tls"
	"errors"
	"github.com/aacebedo/dnsdock/src/servers"
	"github.com/aacebedo/dnsdock/src/utils"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	eventtypes "github.com/docker/engine-api/types/events"
	"github.com/vdemeester/docker-events"
	"golang.org/x/net/context"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// DockerManager is the entrypoint to the docker daemon
type DockerManager struct {
	config *utils.Config
	list   servers.ServiceListProvider
	client *client.Client
	cancel context.CancelFunc
}

// NewDockerManager creates a new DockerManager
func NewDockerManager(c *utils.Config, list servers.ServiceListProvider, tlsConfig *tls.Config) (*DockerManager, error) {
	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	dclient, err := client.NewClient(c.DockerHost, "v1.22", nil, defaultHeaders)

	if err != nil {
		return nil, err
	}

	return &DockerManager{config: c, list: list, client: dclient}, nil
}

// Start starts the DockerManager
func (d *DockerManager) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	startHandler := func(m eventtypes.Message) {
		logger.Debugf("Started container '%s'", m.ID)
		service, err := d.getService(m.ID)
		if err != nil {
			logger.Errorf("%s", err)
		} else {
			d.list.AddService(m.ID, *service)
		}
	}

	stopHandler := func(m eventtypes.Message) {
		logger.Debugf("Stopped container '%s'", m.ID)
		if !d.config.All {
			d.list.RemoveService(m.ID)
		} else {
			logger.Debugf("Stopped container '%s' not removed as --all argument is true", m.ID)
		}
	}

	renameHandler := func(m eventtypes.Message) {
		oldName, ok := m.Actor.Attributes["oldName"]
		name, ok2 := m.Actor.Attributes["oldName"]
		if ok && ok2 {
			logger.Debugf("Renamed container '%s' into '%s'", oldName, name)
			d.list.RemoveService(oldName)
			service, err := d.getService(m.ID)
			if err != nil {
				logger.Errorf("%s", err)
			} else {
				d.list.AddService(m.ID, *service)
			}
		}
	}

	destroyHandler := func(m eventtypes.Message) {
		logger.Debugf("Destroy container '%s'", m.ID)
		if d.config.All {
			d.list.RemoveService(m.ID)
		}
	}

	eventHandler := events.NewHandler(events.ByAction)
	eventHandler.Handle("start", startHandler)
	eventHandler.Handle("stop", stopHandler)
	eventHandler.Handle("die", stopHandler)
	eventHandler.Handle("kill", stopHandler)
	eventHandler.Handle("destroy", destroyHandler)
	eventHandler.Handle("rename", renameHandler)

	events.MonitorWithHandler(ctx, d.client, types.EventsOptions{}, eventHandler)

	containers, err := d.client.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return errors.New("Error getting containers: " + err.Error())
	}

	for _, container := range containers {
		service, err := d.getService(container.ID)
		if err != nil {
			logger.Errorf("%s", err)
			continue
		}
		d.list.AddService(container.ID, *service)
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

	service := servers.NewService()
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
	var region, tags string
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
			tags = v
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

	if len(tags) > 0 {
		// Currently only supports the first tag, but should be able to create multiple services,
		// i.e. an array for in.Name, to support multiple service creation
		in.Name = strings.Split(tags, ",")[0] + "." + in.Name
	}
	if len(region) > 0 {
		in.Image = region + "." + in.Image
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
