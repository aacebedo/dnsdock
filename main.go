package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	help := flag.Bool("help", false, "Show this message")

	config := NewConfig()

	flag.StringVar(&config.nameserver, "nameserver", config.nameserver, "DNS server for unmatched requests")
	flag.StringVar(&config.dnsAddr, "dns", config.dnsAddr, "Listen DNS requests on this address")
	flag.StringVar(&config.domain, "domain", config.domain, "Domain that is appended to all requests")
	flag.StringVar(&config.environment, "environment", config.environment, "Optional context before domain suffix")
	flag.StringVar(&config.dockerHost, "docker", config.dockerHost, "Path to the docker socket")

	flag.Parse()

	if *help {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
		return
	}

	fmt.Printf("%#v\n", config)
}
