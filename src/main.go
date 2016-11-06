package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
)

var version string

func main() {

	var cmdLine CommandLine
	config, err := cmdLine.ParseParameters(os.Args[1:])
	if err != nil {
		logger.Fatalf(err.Error())
	}
	verbosity := 0
	if config.quiet == false {
		if config.verbose == false {
			verbosity = 1
		} else {
			verbosity = 2
		}
	}
	err = InitLoggers(verbosity)
  if err != nil {
		logger.Fatalf("Unable to initialize loggers! %s", err.Error())
	}
  
	dnsServer := NewDNSServer(config)

	var tlsConfig *tls.Config
	if config.tlsVerify {
		clientCert, err := tls.LoadX509KeyPair(config.tlsCert, config.tlsKey)
		if err != nil {
			logger.Fatalf("Error: '%s'", err)
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
			logger.Fatalf("Error: '%s'", err)
		}
	}
	
	docker, err := NewDockerManager(config, dnsServer, tlsConfig)
	if err != nil {
		logger.Fatalf("Error: '%s'", err)
	}
	if err := docker.Start(); err != nil {
		logger.Fatalf("Error: '%s'", err)
	}
  
	httpServer := NewHTTPServer(config, dnsServer)
	go func() {
		if err := httpServer.Start(); err != nil {
			logger.Fatalf("Error: '%s'", err)
		}
	}()

	if err := dnsServer.Start(); err != nil {
		logger.Fatalf("Error: '%s'", err)
	}

}
