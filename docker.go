package main

import (
	"errors"
	"github.com/samalba/dockerclient"
	"log"
	"net"
	"regexp"
	"strings"
)

type DockerManager struct {
	config *Config
	list   ServiceListProvider
	docker *dockerclient.DockerClient
}

func NewDockerManager(c *Config, list ServiceListProvider) (*DockerManager, error) {
	docker, err := dockerclient.NewDockerClient(c.dockerHost, nil)
	if err != nil {
		return nil, err
	}

	return &DockerManager{config: c, list: list, docker: docker}, nil
}

func (d *DockerManager) Start() error {
	containers, err := d.docker.ListContainers(false)
	if err != nil {
		return errors.New("Error connecting to docker socket: " + err.Error())
	}

	for _, container := range containers {
		service, err := d.getService(container.Id)
		if err != nil {
			log.Println(err)
			continue
		}
		d.list.AddService(container.Id, *service)
	}

	return nil
}

func (d *DockerManager) getService(id string) (*Service, error) {
	inspect, err := d.docker.InspectContainer(id)
	if err != nil {
		return nil, err
	}

	service := NewService()

	service.Image = getImageName(inspect.Config.Image)
	if imageNameIsSHA(service.Image, inspect.Image) {
		log.Println("Warning: Can't route ", id[:10], ", image", service.Image, "is not a tag.")
		service.Image = ""
	}
	service.Name = cleanContainerName(inspect.Name)
	service.Ip = net.ParseIP(inspect.NetworkSettings.IpAddress)

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
