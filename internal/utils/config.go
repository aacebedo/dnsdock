/* config.go
 *
 * Copyright (C) 2016 Alexandre ACEBEDO
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file for details.
 */

package utils

import (
	"os"
	"strings"
)

// Domain represents a domain
type Domain []string

// NewDomain creates a new domain
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

// Config contains DNSDock configuration
type Config struct {
	Nameservers nameservers
	DnsAddr     string
	Domain      Domain
	DockerHost  string
	TlsVerify   bool
	TlsCaCert   string
	TlsCert     string
	TlsKey      string
	HttpAddr    string
	Ttl         int
	ForceTtl    bool
	CreateAlias bool
	Verbose     bool
	Quiet       bool
	All         bool
}

// NewConfig creates a new config
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
		Nameservers: nameservers{"8.8.8.8:53"},
		DnsAddr:     ":53",
		Domain:      NewDomain("docker"),
		DockerHost:  dockerHost,
		HttpAddr:    ":80",
		CreateAlias: false,
		TlsVerify:   tlsVerify,
		TlsCaCert:   dockerCerts + "/ca.pem",
		TlsCert:     dockerCerts + "/cert.pem",
		TlsKey:      dockerCerts + "/key.pem",
		Verbose:     false,
		Quiet:       false,
		All:         false,
		ForceTtl:    false,
		Ttl:         0,
	}

}
