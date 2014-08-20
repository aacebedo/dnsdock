package main

import (
	"errors"
	"github.com/miekg/dns"
	"log"
	"net"
	"strings"
	"sync"
)

type Service struct {
	Name  string
	Image string
	Ip    net.IP
	Ttl   int
}

func NewService() (s *Service) {
	s = &Service{Ttl: -1}
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
		log.Println("Added service:", id, service)
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
		log.Println("Stopped service:", id)
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

	m := new(dns.Msg)
	m.Answer = make([]dns.RR, 0, 2)

	query := r.Question[0].Name
	if query[len(query)-1] == '.' {
		query = query[:len(query)-1]
	}

	for service := range s.queryServices(query) {
		rr := new(dns.A)

		var ttl int
		if service.Ttl != -1 {
			ttl = service.Ttl
		} else {
			ttl = 0
		}

		rr.Hdr = dns.RR_Header{
			Name:   r.Question[0].Name,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    uint32(ttl),
		}
		rr.A = service.Ip
		m.Answer = append(m.Answer, rr)
	}

	m.SetReply(r)
	w.WriteMsg(m)
}

func (s *DNSServer) queryServices(query string) chan *Service {
	c := make(chan *Service)

	go func() {
		query := strings.Split(strings.ToLower(query), ".")

		defer s.lock.RUnlock()
		s.lock.RLock()

		for _, service := range s.services {
			tests := [][]string{
				s.config.domain,
				strings.Split(service.Image, "."),
				strings.Split(service.Name, "."),
			}

			for i, q := 0, query; ; i++ {
				if len(q) == 0 || i > 2 {
					c <- service
					break
				}

				var matches bool
				if matches, q = matchSuffix(q, tests[i]); !matches {
					break
				}
			}

		}

		close(c)

	}()

	return c

}

func matchSuffix(str, sfx []string) (matches bool, remainder []string) {
	for i := 1; i <= len(sfx); i++ {
		if len(str) < i {
			return true, nil
		}
		if sfx[len(sfx)-i] != str[len(str)-i] && str[len(str)-i] != "*" {
			return false, nil
		}
	}
	return true, str[:len(str)-len(sfx)]
}
