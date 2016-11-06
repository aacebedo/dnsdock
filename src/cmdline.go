package main

import (
	"fmt"
	"gopkg.in/alecthomas/kingpin.v2"
	"strconv"
)

const (
	VERSION = "0.9.1"
)

type CommandLine struct{}

func (self *CommandLine) ParseParameters(rawParams []string) (res *Config, err error) {

	res = NewConfig()

	app := kingpin.New("dnsdock", "Automatic DNS for docker containers.")
	app.Version(VERSION)
	app.HelpFlag.Short('h')
	nameservers := app.Flag("nameserver", "Comma separated list of DNS server(s) for unmatched requests").Strings()
	dns := app.Flag("dns", "Listen DNS requests on this address").Default(res.dnsAddr).Short('d').String()
	http := app.Flag("http", "Listen HTTP requests on this address").Default(res.httpAddr).Short('t').String()
	domain := app.Flag("domain", "Domain that is appended to all requests").Default(res.domain.String()).String()
	environment := app.Flag("environment", "Optional context before domain suffix").Default("").String()
	docker := app.Flag("docker", "Path to the docker socket").Default(res.dockerHost).String()
	tlsverify := app.Flag("tlsverify", "Enable mTLS when connecting to docker").Default(strconv.FormatBool(res.tlsVerify)).Bool()
	tlscacert := app.Flag("tlscacert", "Path to CA certificate").Default(res.tlsCaCert).String()
	tlscert := app.Flag("tlscert", "Path to Client certificate").Default(res.tlsCert).String()
	tlskey := app.Flag("tlskey", "Path to client certificate private key").Default(res.tlsKey).String()
	ttl := app.Flag("ttl", "TTL for matched requests").Default(strconv.FormatInt(int64(res.ttl), 10)).Int()
	createAlias := app.Flag("alias", "Automatically create an alias with just the container name.").Default(strconv.FormatBool(res.createAlias)).Bool()
	verbose := app.Flag("verbose", "Verbose mode.").Default(strconv.FormatBool(res.verbose)).Short('v').Bool()
	quiet := app.Flag("quiet", "Quiet mode.").Default(strconv.FormatBool(res.quiet)).Short('q').Bool()

	kingpin.MustParse(app.Parse(rawParams))

	res.verbose = *verbose
	res.quiet = *quiet
	res.nameserver = *nameservers
	res.dnsAddr = *dns
	res.httpAddr = *http
	res.domain = NewDomain(fmt.Sprintf("%s.%s", *environment, *domain))
	res.dockerHost = *docker
	res.tlsVerify = *tlsverify
	res.tlsCaCert = *tlscacert
	res.tlsCert = *tlscert
	res.tlsKey = *tlskey
	res.ttl = *ttl
	res.createAlias = *createAlias
	return
}
