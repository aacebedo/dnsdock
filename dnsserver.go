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
	mux      *dns.ServeMux
	services map[string]*Service
	lock     *sync.RWMutex
}

func NewDNSServer(c *Config) *DNSServer {
	s := &DNSServer{
		config:   c,
		services: make(map[string]*Service),
		lock:     &sync.RWMutex{},
	}

	s.mux = dns.NewServeMux()
	s.mux.HandleFunc(c.domain.String()+".", s.handleRequest)
	s.mux.HandleFunc("in-addr.arpa.", s.handleReverseRequest)
	s.mux.HandleFunc(".", s.handleForward)

	s.server = &dns.Server{Addr: c.dnsAddr, Net: "udp", Handler: s.mux}

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

	for _, alias := range service.Aliases {
		s.mux.HandleFunc(alias+".", s.handleRequest)
	}

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

	for _, alias := range s.services[id].Aliases {
		s.mux.HandleRemove(alias + ".")
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

func (s *DNSServer) handleForward(w dns.ResponseWriter, r *dns.Msg) {
	// Otherwise just forward the request to another server
	c := new(dns.Client)
	if in, _, err := c.Exchange(r, s.config.nameserver); err != nil {
		log.Print(err)

		m := new(dns.Msg)
		m.SetReply(r)
		m.Ns = s.createSOA()
		m.SetRcode(r, dns.RcodeRefused) // REFUSED

		w.WriteMsg(m)
	} else {
		w.WriteMsg(in)
	}
}

func (s *DNSServer) makeServiceA(n string, service *Service) dns.RR {
	rr := new(dns.A)

	var ttl int
	if service.Ttl != -1 {
		ttl = service.Ttl
	} else {
		ttl = s.config.ttl
	}

	rr.Hdr = dns.RR_Header{
		Name:   n,
		Rrtype: dns.TypeA,
		Class:  dns.ClassINET,
		Ttl:    uint32(ttl),
	}

	rr.A = service.Ip

	return rr
}

func (s *DNSServer) makeServiceMX(n string, service *Service) dns.RR {
	rr := new(dns.MX)

	var ttl int
	if service.Ttl != -1 {
		ttl = service.Ttl
	} else {
		ttl = s.config.ttl
	}

	rr.Hdr = dns.RR_Header{
		Name:   n,
		Rrtype: dns.TypeMX,
		Class:  dns.ClassINET,
		Ttl:    uint32(ttl),
	}

	rr.Mx = n

	return rr
}

func (s *DNSServer) handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)

	// Send empty response for empty requests
	if len(r.Question) == 0 {
		m.Ns = s.createSOA()
		w.WriteMsg(m)
		return
	}

	// respond to SOA requests
	if r.Question[0].Qtype == dns.TypeSOA {
		m.Answer = s.createSOA()
		w.WriteMsg(m)
		return
	}

	m.Answer = make([]dns.RR, 0, 2)
	query := r.Question[0].Name

	// trim off any trailing dot
	if query[len(query)-1] == '.' {
		query = query[:len(query)-1]
	}

	for service := range s.queryServices(query) {
		var rr dns.RR
		switch r.Question[0].Qtype {
		case dns.TypeA:
			rr = s.makeServiceA(r.Question[0].Name, service)
		case dns.TypeMX:
			rr = s.makeServiceMX(r.Question[0].Name, service)
		default:
			// this query type isn't supported, but we do have
			// a record with this name. Per RFC 4074 sec. 3, we
			// immediately return an empty NOERROR reply.
			m.Ns = s.createSOA()
			m.MsgHdr.Authoritative = true
			w.WriteMsg(m)
			return
		}

		m.Answer = append(m.Answer, rr)
	}

	// We didn't find a record corresponding to the query
	if len(m.Answer) == 0 {
		m.Ns = s.createSOA()
		m.SetRcode(r, dns.RcodeNameError) // NXDOMAIN
	}

	w.WriteMsg(m)
}

func (s *DNSServer) handleReverseRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)

	// Send empty response for empty requests
	if len(r.Question) == 0 {
		m.Ns = s.createSOA()
		w.WriteMsg(m)
		return
	}

	m.Answer = make([]dns.RR, 0, 2)
	query := r.Question[0].Name

	// trim off any trailing dot
	if query[len(query)-1] == '.' {
		query = query[:len(query)-1]
	}

	for service := range s.queryIp(query) {
		if r.Question[0].Qtype != dns.TypePTR {
			m.Ns = s.createSOA()
			w.WriteMsg(m)
			return
		}

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

	if len(m.Answer) != 0 {
		w.WriteMsg(m)
	} else {
		// We didn't find a record corresponding to the query,
		// try forwarding
		s.handleForward(w, r)
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
				test = append(test, strings.Split(strings.ToLower(service.Name), ".")...)
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
