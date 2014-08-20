package main

import (
	"os"
	"strings"
)

type Domain []string

func NewDomain(s string) Domain {
	s = strings.Replace(s, "..", ".", -1)
	if s[:1] == "." {
		s = s[1:]
	}
	if s[len(s)-1:] == "." {
		s = s[:len(s)-1]
	}
	return Domain(strings.Split(s, "."))
}

func (d *Domain) String() string {
	return strings.Join([]string(*d), ".")
}

type Config struct {
	nameserver string
	dnsAddr    string
	domain     Domain
	dockerHost string
}

func NewConfig() *Config {
	dockerHost := os.Getenv("DOCKER_HOST")
	if len(dockerHost) == 0 {
		dockerHost = "unix://var/run/docker.sock"
	}

	return &Config{
		nameserver: "8.8.8.8:53",
		dnsAddr:    ":53",
		domain:     NewDomain("docker"),
		dockerHost: dockerHost,
	}

}
