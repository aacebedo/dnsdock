package main

import (
	"errors"
	"log"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type Service struct {
	Name    string
	Image   string
	Ip      net.IP
	Ttl     int
	Aliases []string
	Manual  bool
}

func NewService() (s *Service) {
	s = &Service{Ttl: -1, Manual: false}
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
	mux.HandleFunc("arpa.", s.handleRequest)
	mux.HandleFunc(".", s.handleOrForward)

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

	id = s.getExpandedID(id)
	s.services[id] = &service

	if s.config.verbose {
		log.Println("Added service:", id, service)
	}
}

func (s *DNSServer) RemoveService(id string) error {
	defer s.lock.Unlock()
	s.lock.Lock()

	id = s.getExpandedID(id)
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

	id = s.getExpandedID(id)
	if s, ok := s.services[id]; ok {
		return *s, nil
	}
	// Check for a pa
	return *new(Service), errors.New("No such service: " + id)
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

func (s *DNSServer) listDomains(service *Service) chan string {
	c := make(chan string)

	go func() {

		if service.Image == "" {
			c <- service.Name + "." + s.config.domain.String() + "."
		} else {
			domain := service.Image + "." + s.config.domain.String() + "."

			c <- domain
			c <- service.Name + "." + domain
		}

		for _, alias := range service.Aliases {
			c <- alias + "."
		}

		close(c)
	}()

	return c
}

func (s *DNSServer) handleOrForward(w dns.ResponseWriter, r *dns.Msg) {
	m := s.doHandle(w, r)

	// We could build a response, let's return it
	if len(m.Answer) != 0 {
		w.WriteMsg(m)
		return
	}

	// Otherwise just forward the request to another server
	c := new(dns.Client)
	if in, _, err := c.Exchange(r, s.config.nameserver); err != nil {
		log.Print(err)
		w.WriteMsg(new(dns.Msg))
	} else {
		w.WriteMsg(in)
	}
}

func (s *DNSServer) handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := s.doHandle(w, r)

	// We could NOT build a response
	if len(m.Answer) == 0 && len(m.Ns) == 0 {
		m.Ns = s.createSOA()
		m.SetRcode(r, dns.RcodeNameError)
	}

	w.WriteMsg(m)
}

func (s *DNSServer) doHandle(w dns.ResponseWriter, r *dns.Msg) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(r)

	// Send empty response for empty requests
	if len(r.Question) == 0 {
		m.Ns = s.createSOA()
		return m
	}

	switch r.Question[0].Qtype {
	case dns.TypePTR:
		s.handlePTRRequest(r, m)
	case dns.TypeMX:
		s.handleMXRequest(r, m)
	case dns.TypeA:
		s.handleARequest(r, m)
	case dns.TypeSOA:
		m.Answer = s.createSOA()
	default:
		m.Ns = s.createSOA()
		m.SetRcode(r, dns.RcodeNotImplemented)
	}

	return m
}

func (s *DNSServer) handleMXRequest(r *dns.Msg, m *dns.Msg) {
	m.Answer = make([]dns.RR, 0, 2)
	query := r.Question[0].Name

	if query[len(query)-1] == '.' {
		query = query[:len(query)-1]
	}

	for service := range s.queryServices(query) {
		rr := new(dns.MX)

		var ttl int
		if service.Ttl != -1 {
			ttl = service.Ttl
		} else {
			ttl = s.config.ttl
		}

		rr.Hdr = dns.RR_Header{
			Name:   r.Question[0].Name,
			Rrtype: dns.TypeMX,
			Class:  dns.ClassINET,
			Ttl:    uint32(ttl),
		}
		rr.Mx = r.Question[0].Name
		m.Answer = append(m.Answer, rr)
	}
}

func (s *DNSServer) handlePTRRequest(r *dns.Msg, m *dns.Msg) {
	m.Answer = make([]dns.RR, 0, 2)
	query := r.Question[0].Name

	if query[len(query)-1] == '.' {
		query = query[:len(query)-1]
	}

	for service := range s.queryIp(query) {
		var ttl int
		if service.Ttl != -1 {
			ttl = service.Ttl
		} else {
			ttl = s.config.ttl
		}

		for domain := range s.listDomains(service) {
			rr := new(dns.PTR)
			rr.Hdr = dns.RR_Header{
				Name:   r.Question[0].Name,
				Rrtype: dns.TypePTR,
				Class:  dns.ClassINET,
				Ttl:    uint32(ttl),
			}
			rr.Ptr = domain

			m.Answer = append(m.Answer, rr)
		}
	}
}

func (s *DNSServer) handleARequest(r *dns.Msg, m *dns.Msg) {
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
			ttl = s.config.ttl
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
}

func (s *DNSServer) queryIp(query string) chan *Service {
	c := make(chan *Service, 3)
	reversedIp := strings.TrimSuffix(query, ".in-addr.arpa")
	ip := strings.Join(reverse(strings.Split(reversedIp, ".")), ".")

	go func() {
		defer s.lock.RUnlock()
		s.lock.RLock()

		for _, service := range s.services {
			if service.Ip.String() == ip {
				c <- service
			}
		}

		close(c)
	}()

	return c
}

func (s *DNSServer) queryServices(query string) chan *Service {
	c := make(chan *Service, 3)

	go func() {
		query := strings.Split(strings.ToLower(query), ".")

		defer s.lock.RUnlock()
		s.lock.RLock()

		for _, service := range s.services {
			// create the name for this service, skip empty strings
			test := []string{}
			// todo: add some cache to avoid calculating this every time
			if len(service.Name) > 0 {
				test = append(test, strings.Split(service.Name, ".")...)
			}

			if len(service.Image) > 0 {
				test = append(test, strings.Split(service.Image, ".")...)
			}

			test = append(test, s.config.domain...)

			if isPrefixQuery(query, test) {
				c <- service
			}

			// check aliases
			for _, alias := range service.Aliases {
				if isPrefixQuery(query, strings.Split(alias, ".")) {
					c <- service
				}
			}
		}

		close(c)

	}()

	return c

}

// Checks for a partial match for container SHA and outputs it if found.
func (s *DNSServer) getExpandedID(in string) (out string) {
	out = in

	// Hard to make a judgement on small image names.
	if len(in) < 4 {
		return
	}

	if isHex, _ := regexp.MatchString("^[0-9a-f]+$", in); !isHex {
		return
	}

	for id := range s.services {
		if len(id) == 64 {
			if isHex, _ := regexp.MatchString("^[0-9a-f]+$", id); isHex {
				if strings.HasPrefix(id, in) {
					out = id
					return
				}
			}
		}
	}
	return
}

// Ttl is used from config so that not-found result responses are not cached
// for a long time. The other defaults left as is(skydns source) because they
// do not have an use case in this situation.
func (s *DNSServer) createSOA() []dns.RR {
	dom := dns.Fqdn(s.config.domain.String() + ".")
	soa := &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   dom,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    uint32(s.config.ttl)},
		Ns:      "dnsdock." + dom,
		Mbox:    "dnsdock.dnsdock." + dom,
		Serial:  uint32(time.Now().Truncate(time.Hour).Unix()),
		Refresh: 28800,
		Retry:   7200,
		Expire:  604800,
		Minttl:  uint32(s.config.ttl),
	}
	return []dns.RR{soa}
}

// isPrefixQuery is used to determine whether "query" is a potential prefix
// query for "name". It allows for wildcards (*) in the query. However is makes
// one exception to accomodate the desired behavior we wish from dnsdock,
// namely, the query may be longer than "name" and still be a valid prefix
// query for "name".
// Examples:
//   foo.bar.baz.qux is a valid query for bar.baz.qux (longer prefix is okay)
//   foo.*.baz.qux   is a valid query for bar.baz.qux (wildcards okay)
//   *.baz.qux       is a valid query for baz.baz.qux (wildcard prefix okay)
func isPrefixQuery(query, name []string) bool {
	for i, j := len(query)-1, len(name)-1; i >= 0 && j >= 0; i, j = i-1, j-1 {
		if query[i] != name[j] && query[i] != "*" {
			return false
		}
	}
	return true
}

func reverse(input []string) []string {
	if len(input) == 0 {
		return input
	}

	return append(reverse(input[1:]), input[0])
}
