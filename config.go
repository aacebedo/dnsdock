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

// type that knows how to parse CSV strings and store the values in a slice
type nameservers []string

func (n *nameservers) String() string {
	return strings.Join(*n, " ")
}

// accumulate the CSV string of nameservers
func (n *nameservers) Set(value string) error {
	*n = nil
	for _, ns := range strings.Split(value, ",") {
		ns = strings.Trim(ns, " ")
		*n = append(*n, ns)
	}

	return nil
}

type Config struct {
	nameserver nameservers
	dnsAddr    string
	domain     Domain
	dockerHost string
	tlsVerify  bool
	tlsCaCert  string
	tlsCert    string
	tlsKey     string
	verbose    bool
	httpAddr   string
	ttl        int
}

func NewConfig() *Config {
	dockerHost := os.Getenv("DOCKER_HOST")
	if len(dockerHost) == 0 {
		dockerHost = "unix:///var/run/docker.sock"
	}
	tlsVerify := len(os.Getenv("DOCKER_TLS_VERIFY")) != 0
	dockerCerts := os.Getenv("DOCKER_CERT_PATH")
	if len(dockerCerts) == 0 {
		dockerCerts = os.Getenv("HOME") + "/.docker"
	}

	return &Config{
		nameserver: nameservers{"8.8.8.8:53"},
		dnsAddr:    ":53",
		domain:     NewDomain("docker"),
		dockerHost: dockerHost,
		httpAddr:   ":80",
		tlsVerify:  tlsVerify,
		tlsCaCert:  dockerCerts + "/ca.pem",
		tlsCert:    dockerCerts + "/cert.pem",
		tlsKey:     dockerCerts + "/key.pem",
	}

}
