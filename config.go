package main

import "os"

type Config struct {
	nameserver  string
	dnsAddr     string
	domain      string
	environment string
	dockerHost  string
}

func NewConfig() *Config {
	dockerHost := os.Getenv("DOCKER_HOST")
	if len(dockerHost) == 0 {
		dockerHost = "unix://var/run/docker.sock"
	}

	return &Config{
		nameserver:  "8.8.8.8:53",
		dnsAddr:     ":53",
		domain:      "docker",
		environment: "",
		dockerHost:  dockerHost,
	}

}
