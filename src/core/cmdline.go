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

const (
	// VERSION dnsdock version
	VERSION = "1.16.1"
)

// CommandLine structure handling parameter parsing
type CommandLine struct{}

// ParseParameters Parse parameters
func (cmdline *CommandLine) ParseParameters(rawParams []string) (res *utils.Config, err error) {
	res = utils.NewConfig()

	app := kingpin.New("dnsdock", "Automatic DNS for docker containers.")
	app.Version(VERSION)
	app.HelpFlag.Short('h')

	nameservers := app.Flag("nameserver", "Comma separated list of DNS server(s) for unmatched requests").Default("8.8.8.8:53").Strings()
	dns := app.Flag("dns", "Listen DNS requests on this address").Default(res.DnsAddr).Short('d').String()
	http := app.Flag("http", "Listen HTTP requests on this address").Default(res.HttpAddr).Short('t').String()
	domain := app.Flag("domain", "Domain that is appended to all requests").Default(res.Domain.String()).String()
	environment := app.Flag("environment", "Optional context before domain suffix").Default("").String()
	docker := app.Flag("docker", "Path to the docker socket").Default(res.DockerHost).String()
	tlsverify := app.Flag("tlsverify", "Enable mTLS when connecting to docker").Default(strconv.FormatBool(res.TlsVerify)).Bool()
	tlscacert := app.Flag("tlscacert", "Path to CA certificate").Default(res.TlsCaCert).String()
	tlscert := app.Flag("tlscert", "Path to Client certificate").Default(res.TlsCert).String()
	tlskey := app.Flag("tlskey", "Path to client certificate private key").Default(res.TlsKey).String()
	ttl := app.Flag("ttl", "TTL for matched requests").Default(strconv.FormatInt(int64(res.Ttl), 10)).Int()
	createAlias := app.Flag("alias", "Automatically create an alias with just the container name.").Default(strconv.FormatBool(res.CreateAlias)).Bool()
	verbose := app.Flag("verbose", "Verbose mode.").Default(strconv.FormatBool(res.Verbose)).Short('v').Bool()
	quiet := app.Flag("quiet", "Quiet mode.").Default(strconv.FormatBool(res.Quiet)).Short('q').Bool()

	kingpin.MustParse(app.Parse(rawParams))

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
