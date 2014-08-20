package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	help := flag.Bool("help", false, "Show this message")

	config := NewConfig()

	flag.StringVar(&config.nameserver, "nameserver", config.nameserver, "DNS server for unmatched requests")
	flag.StringVar(&config.dnsAddr, "dns", config.dnsAddr, "Listen DNS requests on this address")
	flag.StringVar(&config.httpAddr, "http", config.httpAddr, "Listen HTTP requests on this address")
	domain := flag.String("domain", config.domain.String(), "Domain that is appended to all requests")
	environment := flag.String("environment", "", "Optional context before domain suffix")
	flag.StringVar(&config.dockerHost, "docker", config.dockerHost, "Path to the docker socket")
	flag.BoolVar(&config.verbose, "verbose", true, "Verbose output")
	flag.IntVar(&config.ttl, "ttl", config.ttl, "TTL for matched requests")

	flag.Parse()

	if *help {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
		return
	}

	config.domain = NewDomain(*environment + "." + *domain)

	fmt.Printf("%#v\n", config)

	dnsServer := NewDNSServer(config)

	docker, err := NewDockerManager(config, dnsServer)
	if err != nil {
		log.Fatal(err)
	}
	if err := docker.Start(); err != nil {
		log.Fatal(err)
	}

	httpServer := NewHTTPServer(config, dnsServer)
	go func() {
		if err := httpServer.Start(); err != nil {
			log.Fatal(err)
		}
	}()

	if err := dnsServer.Start(); err != nil {
		log.Fatal(err)
	}

}
