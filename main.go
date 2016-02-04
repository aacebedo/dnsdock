package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

var version string

func main() {
	help := flag.Bool("help", false, "Show this message")

	config := NewConfig()

	flag.StringVar(&config.nameserver, "nameserver", config.nameserver, "DNS server for unmatched requests")
	flag.StringVar(&config.dnsAddr, "dns", config.dnsAddr, "Listen DNS requests on this address")
	flag.StringVar(&config.httpAddr, "http", config.httpAddr, "Listen HTTP requests on this address")
	domain := flag.String("domain", config.domain.String(), "Domain that is appended to all requests")
	environment := flag.String("environment", "", "Optional context before domain suffix")
	flag.StringVar(&config.dockerHost, "docker", config.dockerHost, "Path to the docker socket")
	flag.BoolVar(&config.tlsVerify, "tlsverify", false, "Enable mTLS when connecting to docker")
	flag.StringVar(&config.tlsCaCert, "tlscacert", config.tlsCaCert, "Path to CA certificate")
	flag.StringVar(&config.tlsCert, "tlscert", config.tlsCert, "Path to client certificate")
	flag.StringVar(&config.tlsKey, "tlskey", config.tlsKey, "Path to client certificate private key")
	flag.BoolVar(&config.verbose, "verbose", true, "Verbose output")
	flag.IntVar(&config.ttl, "ttl", config.ttl, "TTL for matched requests")

	var showVersion bool
	if len(version) > 0 {
		flag.BoolVar(&showVersion, "version", false, "Show application version")
	}

	flag.Parse()

	if showVersion {
		fmt.Println("dnsdock", version)
		return
	}

	if *help {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
		return
	}

	config.domain = NewDomain(*environment + "." + *domain)

	dnsServer := NewDNSServer(config)

	var tlsConfig *tls.Config = nil
	if config.tlsVerify {
		clientCert, err := tls.LoadX509KeyPair(config.tlsCert, config.tlsKey)
		if err != nil {
			log.Fatal(err)
		}
		tlsConfig = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{clientCert},
		}
		pemData, err := ioutil.ReadFile(config.tlsCaCert)
		if err == nil {
			rootCert := x509.NewCertPool()
			rootCert.AppendCertsFromPEM(pemData)
			tlsConfig.RootCAs = rootCert
		} else {
			log.Print(err)
		}
	}
	docker, err := NewDockerManager(config, dnsServer, tlsConfig)
	if err != nil {
		log.Fatal(err)
	}
	if err := docker.Start(); err != nil {
		log.Fatal(err)
	}

	httpServer := NewHTTPServer(config, dnsServer, docker)
	go func() {
		if err := httpServer.Start(); err != nil {
			log.Fatal(err)
		}
	}()

	if err := dnsServer.Start(); err != nil {
		log.Fatal(err)
	}

}
