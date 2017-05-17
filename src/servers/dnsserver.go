/* dnsserver.go
 *
 * Copyright (C) 2016 Alexandre ACEBEDO
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file for details.
 */

package servers

import (
	"errors"
	"fmt"
	"github.com/aacebedo/dnsdock/src/utils"
	"github.com/miekg/dns"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Service represents a container and an attached DNS record
type Service struct {
	Name    string
	Image   string
	IPs     []net.IP
	TTL     int
	Aliases []string
}

// NewService creates a new service
func NewService() (s *Service) {
	s = &Service{TTL: -1}
	return
}
func (s Service) String() string {
	return fmt.Sprintf(` Name:    %s
                       Aliases: %s
                       IPs:     %s
                       TTL:     %d
        `, s.Name, s.Aliases, s.IPs, s.TTL)
}

// ServiceListProvider represents the entrypoint to get containers
type ServiceListProvider interface {
	AddService(string, Service)
	RemoveService(string) error
	GetService(string) (Service, error)
	GetAllServices() map[string]Service
}


type DNSSearchTree struct {
  root *dnsTreeNode
  domain utils.Domain
}

func NewDNSSearchTree(domain utils.Domain) *DNSSearchTree {
  return &DNSSearchTree {
    root: &dnsTreeNode{
    		slug:     "",
    		children: make(map[string]*dnsTreeNode),
    		services: make([]*Service, 0),
    },
    domain: domain,
  }
}

func (cache *DNSSearchTree) Add(service *Service) {
  serviceNames := cache.serviceNames(service)
	for i := 0; i < len(serviceNames); i++ {
		cache.root.addServicePath(service, serviceNames[i])
	}
}

func (cache *DNSSearchTree) Remove(service *Service) {
  serviceNames := cache.serviceNames(service)
	for i := 0; i < len(serviceNames); i++ {
		cache.root.removeServicePath(serviceNames[i])
	}
}

func (cache *DNSSearchTree) Search(query string) []*Service {
  return cache.root.search(strings.Split(query, "."))
}

func (cache *DNSSearchTree) serviceNames(service *Service) [][]string {
  result := [][]string {}

  nameParts := []string{}
  if len(service.Name) > 0 {
    nameParts = append(nameParts, strings.Split(strings.ToLower(service.Name), ".")...)
  }
  if len(service.Image) > 0 {
    nameParts = append(nameParts, strings.Split(service.Image, ".")...)
  }
  nameParts = append(nameParts, cache.domain...)
  result = append(result, nameParts)

  for _, alias := range service.Aliases {
    result = append(result, strings.Split(alias, "."))
  }
  return result
}

type dnsTreeNode struct {
	slug     string
	children map[string]*dnsTreeNode
	services []*Service
}

func (node *dnsTreeNode) addServicePath(service *Service, nameParts []string) {
	currentNode := node
	for i := len(nameParts) - 1; i >= 0; i-- {
		namePart := nameParts[i]

		v, ok := currentNode.children[namePart]
		if !ok {
			var services []*Service
			if i == 0 {
				services = []*Service{service}
			} else {
				services = make([]*Service, 0)
			}

			v = &dnsTreeNode{
				slug:     namePart,
				children: make(map[string]*dnsTreeNode),
				services: services,
			}
			currentNode.children[namePart] = v
		}
		currentNode = v
	}
}

func (node *dnsTreeNode) removeServicePath(nameParts []string) {
  currentNode := node
  for i := len(nameParts) - 1; i >= 0; i-- {
    namePart := nameParts[i]

    v, ok := currentNode.children[namePart]
    if !ok {
      return
    }
    if i == 0 {
      if len(v.children) == 0 {
        delete(currentNode.children, namePart)
      } else {
        v.services = []*Service{}
      }
    } else {
      currentNode = v
    }
  }
}

func (node *dnsTreeNode) search(nameParts []string) []*Service {
	currentNode := node
	for i := len(nameParts) - 1; i >= 0; i-- {
		namePart := nameParts[i]

		if i > 0 {
			v, ok := currentNode.children[namePart]
			if !ok {
				return currentNode.services
			} else {
				currentNode = v
			}
		} else {
			if namePart == "*" {
				// return collection of subdomain services
				return currentNode.populateServices()
			} else {
				v, ok := currentNode.children[namePart]
				if ok {
					return v.services
				} else {
					return currentNode.services
				}
			}
		}
	}
	return []*Service{}
}

func (node *dnsTreeNode) populateServices() []*Service {
  result := []*Service{}
  result = append(result, node.services...)
  for _, child := range node.children {
    result = append(result, child.populateServices()...)
  }
  return result
}

// DNSServer represents a DNS server
type DNSServer struct {
	config   *utils.Config
	server   *dns.Server
	mux      *dns.ServeMux
	services map[string]*Service
	lock     *sync.RWMutex
	dnsTree  *DNSSearchTree
}

// NewDNSServer create a new DNSServer
func NewDNSServer(c *utils.Config) *DNSServer {
	s := &DNSServer{
		config:   c,
		services: make(map[string]*Service),
		lock:     &sync.RWMutex{},
		dnsTree: NewDNSSearchTree(c.Domain),
	}

	logger.Debugf("Handling DNS requests for '%s'.", c.Domain.String())

	s.mux = dns.NewServeMux()
	s.mux.HandleFunc(c.Domain.String()+".", s.handleRequest)
	s.mux.HandleFunc("in-addr.arpa.", s.handleReverseRequest)
	s.mux.HandleFunc(".", s.handleForward)

	s.server = &dns.Server{Addr: c.DnsAddr, Net: "udp", Handler: s.mux}

	return s
}

// Start starts the DNSServer
func (s *DNSServer) Start() error {
	return s.server.ListenAndServe()
}

// Stop stops the DNSServer
func (s *DNSServer) Stop() {
	s.server.Shutdown()
}

// AddService adds a new container and thus new DNS records
func (s *DNSServer) AddService(id string, service Service) {
	if len(service.IPs) > 0 {
		defer s.lock.Unlock()
		s.lock.Lock()

		id = s.getExpandedID(id)
		s.services[id] = &service
		s.dnsTree.Add(&service)

		logger.Debugf(`Added service: '%s'
                      %s`, id, service)

		for _, alias := range service.Aliases {
			logger.Debugf("Handling DNS requests for '%s'.", alias)
			s.mux.HandleFunc(alias+".", s.handleRequest)
		}
	} else {
		logger.Warningf("Service '%s' ignored: No IP provided:", id, id)
	}
}

// RemoveService removes a new container and thus DNS records
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

	s.dnsTree.Remove(s.services[id])
	delete(s.services, id)

	logger.Debugf("Removed service '%s'", id)

	return nil
}

// GetService reads a service from the repository
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

// GetAllServices reads all services from the repository
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
			c <- service.Name + "." + s.config.Domain.String() + "."
		} else {
			domain := service.Image + "." + s.config.Domain.String() + "."

			c <- service.Name + "." + domain
			c <- domain
		}

		for _, alias := range service.Aliases {
			c <- alias + "."
		}

		close(c)
	}()

	return c
}

func (s *DNSServer) handleForward(w dns.ResponseWriter, r *dns.Msg) {

	logger.Debugf("Using DNS forwarding for '%s'", r.Question[0].Name)
	logger.Debugf("Forwarding DNS nameservers: %s", s.config.Nameservers.String())

	// Otherwise just forward the request to another server
	c := new(dns.Client)

	// look at each Nameserver, stop on success
	for i := range s.config.Nameservers {
		logger.Debugf("Using Nameserver %s", s.config.Nameservers[i])

		in, _, err := c.Exchange(r, s.config.Nameservers[i])
		if err == nil {
			if s.config.ForceTtl {
				logger.Debugf("Forcing Ttl value of the forwarded response")
				for _, rr := range in.Answer {
					rr.Header().Ttl = uint32(s.config.Ttl)
				}
			}
			w.WriteMsg(in)
			return
		}

		if i == (len(s.config.Nameservers) - 1) {
			logger.Warningf("DNS fowarding failed: no more nameservers to try")

			// Send failure reply
			m := new(dns.Msg)
			m.SetReply(r)
			m.Ns = s.createSOA()
			m.SetRcode(r, dns.RcodeRefused) // REFUSED
			w.WriteMsg(m)

		} else {
			logger.Debugf("DNS fowarding failed: trying next Nameserver...")
		}
	}
}

func (s *DNSServer) makeServiceA(n string, service *Service) dns.RR {
	rr := new(dns.A)

	var ttl int
	if service.TTL != -1 {
		ttl = service.TTL
	} else {
		ttl = s.config.Ttl
	}

	rr.Hdr = dns.RR_Header{
		Name:   n,
		Rrtype: dns.TypeA,
		Class:  dns.ClassINET,
		Ttl:    uint32(ttl),
	}

	if len(service.IPs) != 0 {
		if len(service.IPs) > 1 {
			logger.Warningf("Multiple IP address found for container '%s'. Only the first address will be used", service.Name)
		}
		rr.A = service.IPs[0]
	} else {
		logger.Errorf("No valid IP address found for container '%s' ", service.Name)
	}

	return rr
}

func (s *DNSServer) makeServiceMX(n string, service *Service) dns.RR {
	rr := new(dns.MX)

	var ttl int
	if service.TTL != -1 {
		ttl = service.TTL
	} else {
		ttl = s.config.Ttl
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
	m.RecursionAvailable = true

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

	logger.Debugf("DNS request for query '%s' from remote '%s'", query, w.RemoteAddr())

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

		logger.Debugf("DNS record found for query '%s'", query)

		m.Answer = append(m.Answer, rr)
	}

	// We didn't find a record corresponding to the query
	if len(m.Answer) == 0 {
		m.Ns = s.createSOA()
		m.SetRcode(r, dns.RcodeNameError) // NXDOMAIN
		logger.Debugf("No DNS record found for query '%s'", query)
	}

	w.WriteMsg(m)
}

func (s *DNSServer) handleReverseRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.RecursionAvailable = true

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

	for service := range s.queryIP(query) {
		if r.Question[0].Qtype != dns.TypePTR {
			m.Ns = s.createSOA()
			w.WriteMsg(m)
			return
		}

		var ttl int
		if service.TTL != -1 {
			ttl = service.TTL
		} else {
			ttl = s.config.Ttl
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

func (s *DNSServer) queryIP(query string) chan *Service {
	c := make(chan *Service, 3)
	reversedIP := strings.TrimSuffix(query, ".in-addr.arpa")
	ip := strings.Join(reverse(strings.Split(reversedIP, ".")), ".")

	go func() {
		defer s.lock.RUnlock()
		s.lock.RLock()

		for _, service := range s.services {
			if service.IPs[0].String() == ip {
				c <- service
			}
		}

		close(c)
	}()

	return c
}

func (s *DNSServer) queryServices(query string) chan *Service {
	c := make(chan *Service)

	go func() {
		//query := strings.Split(strings.ToLower(query), ".")

		defer s.lock.RUnlock()
		s.lock.RLock()

		result := s.dnsTree.Search(strings.ToLower(query))
		for i:=0; i < len(result); i++ {
			c <- result[i]
		}
		// for _, service := range s.services {
		// 	// create the name for this service, skip empty strings
		// 	test := []string{}
		// 	// todo: add some cache to avoid calculating this every time
		// 	if len(service.Name) > 0 {
		// 		test = append(test, strings.Split(strings.ToLower(service.Name), ".")...)
		// 	}
		//
		// 	if len(service.Image) > 0 {
		// 		test = append(test, strings.Split(service.Image, ".")...)
		// 	}
		//
		// 	test = append(test, s.config.Domain...)
		//
		// 	if isPrefixQuery(query, test) {
		// 		c <- service
		// 	}
		//
		// 	// check aliases
		// 	for _, alias := range service.Aliases {
		// 		if isPrefixQuery(query, strings.Split(alias, ".")) {
		// 			c <- service
		// 		}
		// 	}
		// }

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

// TTL is used from config so that not-found result responses are not cached
// for a long time. The other defaults left as is(skydns source) because they
// do not have an use case in this situation.
func (s *DNSServer) createSOA() []dns.RR {
	dom := dns.Fqdn(s.config.Domain.String() + ".")
	soa := &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   dom,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    uint32(s.config.Ttl)},
		Ns:      "dnsdock." + dom,
		Mbox:    "dnsdock.dnsdock." + dom,
		Serial:  uint32(time.Now().Truncate(time.Hour).Unix()),
		Refresh: 28800,
		Retry:   7200,
		Expire:  604800,
		Minttl:  uint32(s.config.Ttl),
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
