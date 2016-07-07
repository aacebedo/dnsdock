package main

import (
	"crypto/tls"
	"errors"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	eventtypes "github.com/docker/engine-api/types/events"
	"github.com/docker/engine-api/types/network"
	"github.com/vdemeester/docker-events"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/context"
)

type DockerManager struct {
	config *Config
	list   ServiceListProvider
	client *client.Client
	cancel context.CancelFunc
}

func NewDockerManager(c *Config, list ServiceListProvider, tlsConfig *tls.Config) (*DockerManager, error) {
	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	dclient, err := client.NewClient(c.dockerHost, "v1.22", nil, defaultHeaders)

	if err != nil {
		return nil, err
	}

	return &DockerManager{config: c, list: list, client: dclient}, nil
}

func (d *DockerManager) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	startHandler := func(m eventtypes.Message) {
		log.Println("Started container '", m.ID, "'")
		service, err := d.getService(m.ID)
		if err != nil {
			log.Println(err)
		} else {
			d.list.AddService(m.ID, *service)
		}
	}

	stopHandler := func(m eventtypes.Message) {
		log.Println("Stopped container '", m.ID, "'")
		d.list.RemoveService(m.ID)
	}

	eventHandler := events.NewHandler(events.ByAction)
	eventHandler.Handle("start", startHandler)
	eventHandler.Handle("stop", stopHandler)
	eventHandler.Handle("die", stopHandler)
	eventHandler.Handle("kill", stopHandler)

	events.MonitorWithHandler(ctx, d.client, types.EventsOptions{}, eventHandler)

	containers, err := d.client.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return errors.New("Error getting containers: " + err.Error())
	}

	for _, container := range containers {
		service, err := d.getService(container.ID)
		if err != nil {
			log.Println(err)
			continue
		}
		d.list.AddService(container.ID, *service)
	}

	return nil
}

func (d *DockerManager) Stop() {
	d.cancel()
}

func (d *DockerManager) getService(id string) (*Service, error) {
	desc, err := d.client.ContainerInspect(context.Background(), id)
	if err != nil {
		return nil, err
	}

	service := NewService()
	service.Aliases = make([]string, 0)

	service.Image = getImageName(desc.Config.Image)
	if imageNameIsSHA(service.Image, desc.Image) {
		log.Println("Warning: Can't route ", id[:10], ", image", service.Image, "is not a tag.")
		service.Image = ""
	}
	service.Name = cleanContainerName(desc.Name)
	switch len(desc.NetworkSettings.Networks) {
	case 0:
		log.Println("Warning, no IP address found for container ", desc.Name)
	default:
		v := make([]*network.EndpointSettings, 0, len(desc.NetworkSettings.Networks))
		for _, value := range desc.NetworkSettings.Networks {
			v = append(v, value)
		}
		if len(v) > 1 {
			log.Println("Warning, Multiple IP address found for container ", desc.Name, ". Only the first address will be used")
		}
		service.Ip = net.ParseIP(v[0].IPAddress)
	}

	service = overrideFromEnv(service, splitEnv(desc.Config.Env))
	if service == nil {
		return nil, errors.New("Skipping " + id)
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

func overrideFromEnv(in *Service, env map[string]string) (out *Service) {
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
				in.Ttl = ttl
			}
		}

		if k == "SERVICE_REGION" {
			region = v
		}
	}

	if len(region) > 0 {
		in.Image = in.Image + "." + region
	}
	out = in
	return
}
