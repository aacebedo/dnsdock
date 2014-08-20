package main

import (
	"errors"
	"github.com/miekg/dns"
	"log"
	"net"
	"sync"
)

type Service struct {
	Name  string
	Image string
	IP    net.IP
	TTL   int
}

func NewService() (s *Service) {
	s = &Service{TTL: -1}
	return
}

type ServiceListProvider interface {
	AddService(string, Service)
	RemoveService(string) error
	GetService(string) (Service, error)
	GetAllServices() map[string]Service
}

type DNSServer struct {
	config   *Config
	server   *dns.Server
	services map[string]*Service
	lock     *sync.RWMutex
}

func NewDNSServer(c *Config) *DNSServer {
	s := &DNSServer{
		config:   c,
		services: make(map[string]*Service),
		lock:     &sync.RWMutex{},
	}

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

func (s *DNSServer) AddService(id string, service Service) {
	defer s.lock.Unlock()
	s.lock.Lock()

	s.services[id] = &service

	if s.config.verbose {
		log.Println("Added container:", id, service)
	}
}

func (s *DNSServer) RemoveService(id string) error {
	defer s.lock.Unlock()
	s.lock.Lock()

	if _, ok := s.services[id]; !ok {
		return errors.New("No such service: " + id)
	}

	delete(s.services, id)

	if s.config.verbose {
		log.Println("Stopped container:", id)
	}

	return nil
}

func (s *DNSServer) GetService(id string) (Service, error) {
	defer s.lock.RUnlock()
	s.lock.RLock()

	if s, ok := s.services[id]; !ok {
		return *new(Service), errors.New("No such service: " + id)
	} else {
		return *s, nil
	}
}

func (s *DNSServer) GetAllServices() map[string]Service {
	defer s.lock.RUnlock()
	s.lock.RLock()

	list := make(map[string]Service, len(s.services))
	for id, service := range s.services {
		list[id] = *service
	}

	return list
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
