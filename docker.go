package main

import (
	"crypto/tls"
	"errors"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/samalba/dockerclient"
)

type DockerManager struct {
	config *Config
	list   ServiceListProvider
	docker *dockerclient.DockerClient
}

func NewDockerManager(c *Config, list ServiceListProvider, tlsConfig *tls.Config) (*DockerManager, error) {
	docker, err := dockerclient.NewDockerClient(c.dockerHost, tlsConfig)
	if err != nil {
		return nil, err
	}

	return &DockerManager{config: c, list: list, docker: docker}, nil
}

func (d *DockerManager) Start() error {
	ec := make(chan error)
	d.docker.StartMonitorEvents(d.eventCallback, ec)
	go func() {
		backoff := backoff.NewExponentialBackOff()
		for {
			log.Println("Event error:", <-ec)
			// assume for now that an event error necessarily
			// requires a re-establishment of the monitor stream
			d.docker.StopAllMonitorEvents()

			time.Sleep(backoff.NextBackOff())

			log.Println("Reconnecting")
			d.docker.StartMonitorEvents(d.eventCallback, ec)

			d.Update() // catch up with anything we missed
		}
	}()

	return d.Update()
}

func (d *DockerManager) Update() error {
	containers, err := d.docker.ListContainers(false, false, "")
	if err != nil {
		return errors.New("Error connecting to docker socket: " + err.Error())
	}

	for _, container := range containers {
		service, err := d.getService(container.Id)
		if err != nil {
			log.Println(err)
			continue
		}
		s, err := d.list.GetService(container.Id)
		if err != nil || !s.Manual {
			d.list.AddService(container.Id, *service)
		}
	}

	return nil
}

func (d *DockerManager) Stop() {
	d.docker.StopAllMonitorEvents()
}

func (d *DockerManager) getService(id string) (*Service, error) {
	inspect, err := d.docker.InspectContainer(id)
	if err != nil {
		return nil, err
	}

	service := NewService()
	service.Aliases = make([]string, 0)

	service.Image = getImageName(inspect.Config.Image)
	if imageNameIsSHA(service.Image, inspect.Image) {
		log.Println("Warning: Can't route ", id[:10], ", image", service.Image, "is not a tag.")
		service.Image = ""
	}
	service.Name = cleanContainerName(inspect.Name)
	service.Ip = net.ParseIP(inspect.NetworkSettings.IPAddress)

	service = overrideFromEnv(service, splitEnv(inspect.Config.Env))
	if service == nil {
		return nil, errors.New("Skipping " + id)
	}

	return service, nil
}

func (d *DockerManager) eventCallback(event *dockerclient.Event, ec chan error, args ...interface{}) {
	//log.Printf("Received event: %#v %#v\n", *event, args)

	s, s_err := d.list.GetService(event.Id)

	switch event.Status {
	case "die", "stop", "kill":
		// Errors can be ignored here because there can be no-op events.
		d.list.RemoveService(event.Id)
	case "start", "restart":
		service, err := d.getService(event.Id)
		if err != nil {
			ec <- err
			return
		}

		if s_err != nil || !s.Manual {
			d.list.AddService(event.Id, *service)
		}
	}
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
