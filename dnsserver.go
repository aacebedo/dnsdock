package main

import (
	"github.com/miekg/dns"
)

type DNSServer struct {
	config *Config
	server *dns.Server
}

func NewDNSServer(c *Config) *DNSServer {
	s := &DNSServer{}

	mux := dns.NewServeMux()
	mux.HandleFunc(".", s.forwardRequest)

	s.server = &dns.Server{Addr: c.dnsAddr, Net: "udp", Handler: mux}

	return s
}

func (s *DNSServer) Start() error {
	return s.server.ListenAndServe()
}

func (s *DNSServer) Stop() {
	s.server.Shutdown()
}

func (s *DNSServer) forwardRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	w.WriteMsg(m)
}
