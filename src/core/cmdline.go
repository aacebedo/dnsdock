/* cmdline.go
 *
 * Copyright (C) 2016 Alexandre ACEBEDO
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file for details.
 */

package core

import (
	"fmt"
	"github.com/aacebedo/dnsdock/src/utils"
	"gopkg.in/alecthomas/kingpin.v2"
	"strconv"
)

// CommandLine structure handling parameter parsing
type CommandLine struct{
  app *kingpin.Application
}

func NewCommandLine(version string) (res *CommandLine) {
  res = &CommandLine{}
  res.app = kingpin.New("dnsdock", "Automatic DNS for docker containers.")
	res.app.Version(version)
	res.app.HelpFlag.Short('h')
	return
}

// ParseParameters Parse parameters
func (cmdline *CommandLine) ParseParameters(rawParams []string) (res *utils.Config, err error) {
	res = utils.NewConfig()

	nameservers := cmdline.app.Flag("nameserver", "Comma separated list of DNS server(s) for unmatched requests").Default("8.8.8.8:53").Strings()
	dns := cmdline.app.Flag("dns", "Listen DNS requests on this address").Default(res.DnsAddr).Short('d').String()
	http := cmdline.app.Flag("http", "Listen HTTP requests on this address").Default(res.HttpAddr).Short('t').String()
	domain := cmdline.app.Flag("domain", "Domain that is appended to all requests").Default(res.Domain.String()).String()
	environment := cmdline.app.Flag("environment", "Optional context before domain suffix").Default("").String()
	docker := cmdline.app.Flag("docker", "Path to the docker socket").Default(res.DockerHost).String()
	tlsverify := cmdline.app.Flag("tlsverify", "Enable mTLS when connecting to docker").Default(strconv.FormatBool(res.TlsVerify)).Bool()
	tlscacert := cmdline.app.Flag("tlscacert", "Path to CA certificate").Default(res.TlsCaCert).String()
	tlscert := cmdline.app.Flag("tlscert", "Path to Client certificate").Default(res.TlsCert).String()
	tlskey := cmdline.app.Flag("tlskey", "Path to client certificate private key").Default(res.TlsKey).String()
	ttl := cmdline.app.Flag("ttl", "TTL for matched requests").Default(strconv.FormatInt(int64(res.Ttl), 10)).Int()
	createAlias := cmdline.app.Flag("alias", "Automatically create an alias with just the container name.").Default(strconv.FormatBool(res.CreateAlias)).Bool()
	verbose := cmdline.app.Flag("verbose", "Verbose mode.").Default(strconv.FormatBool(res.Verbose)).Short('v').Bool()
	quiet := cmdline.app.Flag("quiet", "Quiet mode.").Default(strconv.FormatBool(res.Quiet)).Short('q').Bool()

	kingpin.MustParse(cmdline.app.Parse(rawParams))

	res.Verbose = *verbose
	res.Quiet = *quiet
	res.Nameservers = *nameservers
	res.DnsAddr = *dns
	res.HttpAddr = *http
	res.Domain = utils.NewDomain(fmt.Sprintf("%s.%s", *environment, *domain))
	res.DockerHost = *docker
	res.TlsVerify = *tlsverify
	res.TlsCaCert = *tlscacert
	res.TlsCert = *tlscert
	res.TlsKey = *tlskey
	res.Ttl = *ttl
	res.CreateAlias = *createAlias
	return
}
