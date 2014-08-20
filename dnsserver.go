package main

import (
	"github.com/miekg/dns"
	"log"
	"net"
)

type DNSServer struct {
	config *Config
	server *dns.Server
}

func NewDNSServer(c *Config) *DNSServer {
	s := &DNSServer{config: c}

	mux := dns.NewServeMux()
	mux.HandleFunc(c.domain[len(c.domain)-1]+".", s.handleRequest)
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
	c := new(dns.Client)
	if in, _, err := c.Exchange(r, s.config.nameserver); err != nil {
		log.Print(err)
		w.WriteMsg(new(dns.Msg))
	} else {
		w.WriteMsg(in)
	}
}

func (s *DNSServer) handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	// Only care about A requests
	if r.Question[0].Qtype != dns.TypeA {
		s.forwardRequest(w, r)
		return
	}

	answer := new(dns.A)
	answer.Hdr = dns.RR_Header{
		Name:   r.Question[0].Name,
		Rrtype: dns.TypeA,
		Class:  dns.ClassINET,
		Ttl:    0,
	}
	answer.A = net.ParseIP("127.0.0.1")

	m := new(dns.Msg)
	m.Answer = []dns.RR{answer}

	m.SetReply(r)
	w.WriteMsg(m)
}
