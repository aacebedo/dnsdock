/* dnsserver_test.go
 *
 * Copyright (C) 2016 Alexandre ACEBEDO
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file for details.
 */

package servers

import (
	"github.com/aacebedo/dnsdock/src/utils"
	"github.com/miekg/dns"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDNSResponse(t *testing.T) {
	utils.InitLoggers(2)
	const TestAddr = "127.0.0.1:9953"

	config := utils.NewConfig(utils.NewDomain("testdomain"))
	config.DnsAddr = TestAddr

	server := NewDNSServer(config)
	go server.Start()

	// Allow some time for server to start
	time.Sleep(250 * time.Millisecond)

	server.AddService("foo", Service{Name: "foo", Image: "bar", IPs: []net.IP{net.ParseIP("127.0.0.1")}})
	server.AddService("baz", Service{Name: "baz", Image: "bar", IPs: []net.IP{net.ParseIP("127.0.0.1")}, TTL: -1})
	server.AddService("biz", Service{Name: "hey", Image: "", IPs: []net.IP{net.ParseIP("127.0.0.4")}})
	server.AddService("joe", Service{Name: "joe", Image: "", IPs: []net.IP{net.ParseIP("127.0.0.5")}, Aliases: []string{"lala.testdomain", "super-alias", "alias.domain"}})

	var inputs = []struct {
		query    string
		expected int
		qType    string
		rcode    int
	}{
		{"google.com.", -1, "A", dns.RcodeSuccess},
		{"google.com.", -1, "MX", 0},
		{"google.com.", -1, "AAAA", 0}, // google has AAAA records
		{"testdomain.", 0, "A", dns.RcodeNameError},
		{"testdomain.", 0, "MX", dns.RcodeNameError},
		{"*.testdomain.", 5, "A", 0},
		{"*.testdomain.", 5, "MX", 0},
		{"bar.testdomain.", 0, "A", dns.RcodeNameError},
		{"bar.testdomain.", 0, "MX", dns.RcodeNameError},
		{"*.bar.testdomain.", 2, "A", 0},
		{"*.bar.testdomain.", 2, "MX", 0},
		{"bar.testdomain.", 0, "AAAA", dns.RcodeNameError},
		{"foo.testdomain.", 0, "A", dns.RcodeNameError},
		{"foo.testdomain.", 0, "MX", dns.RcodeNameError},
		{"baz.bar.testdomain.", 1, "A", 0},
		{"baz.bar.testdomain.", 1, "MX", 0},
		{"joe.testdomain.", 1, "A", 0},
		{"joe.testdomain.", 1, "MX", 0},
		{"joe.testdomain.", 0, "AAAA", 0},
		{"super-alias.", 1, "A", 0},
		{"super-alias.", 1, "MX", 0},
		{"alias.domain.", 1, "A", 0},
		{"alias.domain.", 1, "MX", 0},
		{"1.0.0.127.in-addr.arpa.", 4, "PTR", 0},         // two services match with two domains each
		{"5.0.0.127.in-addr.arpa.", 4, "PTR", 0},         // one service match with three aliases
		{"4.0.0.127.in-addr.arpa.", 1, "PTR", 0},         // only one service with a single domain
		{"2.0.0.127.in-addr.arpa.", 0, "PTR", 0},         // no match
	}

	c := new(dns.Client)
	for _, input := range inputs {
		t.Log("Query", input.query, input.qType)
		qType := dns.StringToType[input.qType]

		m := new(dns.Msg)
		m.SetQuestion(input.query, qType)
		r, _, err := c.Exchange(m, TestAddr)

		if err != nil {
			t.Error("Error response from the server", err)
			break
		}

		if input.expected > 0 && len(r.Answer) != input.expected {
			t.Error(input, "Expected:", input.expected,
				" Got:", len(r.Answer))
		}

		if input.expected < 0 && len(r.Answer) == 0 {
			t.Error(input, "Expected at least one record but got none")
		}

		if r.Rcode != input.rcode {
			t.Error(input, "Rcode expected:",
				dns.RcodeToString[input.rcode],
				" got:", dns.RcodeToString[r.Rcode])
		}

		for _, a := range r.Answer {
			rrType := dns.Type(a.Header().Rrtype).String()
			if input.qType != rrType {
				t.Error("Did not receive ", input.qType, " resource record")
			} else {
				t.Log("Received expected response RR type", rrType, "code", dns.RcodeToString[input.rcode])
			}
		}
	}
}

func TestServiceManagement(t *testing.T) {
	list := ServiceListProvider(NewDNSServer(utils.NewConfig(utils.NewDomain("testdomain"))))

	if len(list.GetAllServices()) != 0 {
		t.Error("Initial service count should be 0.")
	}

	A := Service{Name: "bar", IPs: []net.IP{net.ParseIP("127.0.0.1")}}
	list.AddService("foo", A)

	if len(list.GetAllServices()) != 1 {
		t.Error("Service count should be 1.")
	}

	A.Name = "baz"

	s1, err := list.GetService("foo")
	if err != nil {
		t.Error("GetService error", err)
	}

	if s1.Name != "bar" {
		t.Error("Expected: bar got:", s1.Name)
	}

	_, err = list.GetService("boo")

	if err == nil {
		t.Error("Request to boo should have failed")
	}

	list.AddService("boo", Service{Name: "boo", IPs: []net.IP{net.ParseIP("127.0.0.1")}})

	all := list.GetAllServices()

	delete(all, "foo")
	s2 := all["boo"]
	s2.Name = "zoo"

	if len(list.GetAllServices()) != 2 {
		t.Error("Local map change should not remove items")
	}

	if s1, _ = list.GetService("boo"); s1.Name != "boo" {
		t.Error("Local map change should not change items")
	}

	err = list.RemoveService("bar")
	if err == nil {
		t.Error("Removing bar should fail")
	}

	err = list.RemoveService("foo")
	if err != nil {
		t.Error("Removing foo failed", err)
	}

	if len(list.GetAllServices()) != 1 {
		t.Error("Item count after remove should be 1")
	}

	list.AddService("416261e74515b7dd1dbd55f35e8625b063044f6ddf74907269e07e9f142bc0df", Service{Name: "mysql", IPs: []net.IP{net.ParseIP("127.0.0.1")}})

	if s1, _ = list.GetService("416261"); s1.Name != "mysql" {
		t.Error("Container can't be found by prefix")
	}

	err = list.RemoveService("416261")
	if err != nil {
		t.Error("Removing 416261 failed", err)
	}

	if len(list.GetAllServices()) != 1 {
		t.Error("Item count after remove should be 1")
	}

}

func TestDNSRequestMatch(t *testing.T) {
	inputs := []struct {
		query, domain string
		expected      int
	}{
		{"*.testdomain", "testdomain", 4},
		{"baz.testdomain", "testdomain.local", 0},
		{"*.testdomain.local", "testdomain.local", 4},
		{"foo.testdomain.local", "testdomain.local", 0},
		{"bar.testdomain.local", "testdomain.local", 0},         // matches [foo, baz].testdomain.local
		{"foo.bar.testdomain.local", "testdomain.local", 1},     // matches foo.bar.testdomain.local
		{"*.local", "testdomain.local", 4},                  // matches All
		{"*.testdomain.local", "testdomain.local", 4},           // matches All
		{"bar.*.local", "testdomain.local", 0},              // matches [foo.bar, baz.bar].testdomain.local
		{"*.*.local", "testdomain.local", 4},                // matches All
		{"foo.*.local", "testdomain.local", 0},              // matches None
		{"bar.*.testdomain.local", "testdomain.local", 0},       // matches qux.testdomain.local
		{"foo.*.testdomain", "testdomain", 0},                   // matches foo.bar.testdomain, qux.testdomain
		{"baz.foo.bar.testdomain.local", "testdomain.local", 1}, // matches foo.bar.testdomain.local
		{"foo.bar", "baz.foo.bar", 0},                   // matches all (catchall prefix)
		{"qux.testdomain.local", "testdomain.local", 1},         // matches qux.testdomain.local
		{"*.qux.testdomain", "testdomain", 1},                   // matches qux.testdomain
	}

	for _, input := range inputs {
		c := utils.NewConfig(utils.NewDomain(input.domain))
		server := NewDNSServer(c)

		server.AddService("foo", Service{Name: "foo", Image: "bar", IPs: []net.IP{net.ParseIP("127.0.0.1")}})
		server.AddService("baz", Service{Name: "baz", Image: "bar", IPs: []net.IP{net.ParseIP("127.0.0.1")}})
		server.AddService("abc", Service{Name: "def", Image: "ghi", IPs: []net.IP{net.ParseIP("127.0.0.1")}})
		server.AddService("qux", Service{Name: "qux", Image: "", IPs: []net.IP{net.ParseIP("127.0.0.1")}})

		t.Log(input.query, input.domain)

		actual := 0
		for _ = range server.queryServices(input.query) {
			actual++
		}

		if actual != input.expected {
			t.Error(input, "Expected:", input.expected, "Got:", actual)
		}
	}
}

func TestDNSRequestMatchNamesWithDots(t *testing.T) {
	inputs := []struct {
		query, domain string
		expected      int
	}{
		//{"foo.boo.bar.zar.testdomain", "testdomain", 1},
		//{"coo.boo.bar.zar.testdomain", "testdomain", 0},
		//{"doo.coo.boo.bar.zar.testdomain", "testdomain", 0},
		//{"zar.testdomain", "testdomain", 0},
		{"*.testdomain", "testdomain", 3},
		//{"baz.bar.zar.testdomain", "testdomain", 1},
		//{"boo.bar.zar.testdomain", "testdomain", 0},
		//{"coo.bar.zar.testdomain", "testdomain", 0},
		//{"quu.testdomain.local", "testdomain.local", 0},
		//{"qux.quu.testdomain.local", "testdomain.local", 1},
		//{"qux.*.testdomain.local", "testdomain.local", 0},
		//{"quz.*.testdomain.local", "testdomain.local", 0},
		//{"quz.quu.testdomain.local", "testdomain.local", 0},
		//{"quz.qux.quu.testdomain.local", "testdomain.local", 1},
	}

	for _, input := range inputs {
		c := utils.NewConfig(utils.NewDomain(input.domain))
		server := NewDNSServer(c)

		server.AddService("boo", Service{Name: "foo.boo", Image: "bar.zar", IPs: []net.IP{net.ParseIP("127.0.0.1")}})
		server.AddService("baz", Service{Name: "baz", Image: "bar.zar", IPs: []net.IP{net.ParseIP("127.0.0.1")}})
		server.AddService("bar", Service{Name: "bar", Image: "zar", IPs: []net.IP{net.ParseIP("127.0.0.1")}})
		server.AddService("qux", Service{Name: "qux.quu", Image: "", IPs: []net.IP{net.ParseIP("127.0.0.1")}})

		t.Log(input.query, input.domain)
		actual := 0
		for v := range server.queryServices(input.query) {
			logger.Debugf("%s", v.Name)
			actual++
		}

		if actual != input.expected {
			t.Error(input, "Expected:", input.expected, "Got:", actual)
		}
	}
}

func TestgetExpandedID(t *testing.T) {
	server := NewDNSServer(utils.NewConfig(utils.NewDomain("testdomain")))

	server.AddService("416261e74515b7dd1dbd55f35e8625b063044f6ddf74907269e07e9f142bc0df", Service{})
	server.AddService("316261e74515b7dd1dbd55f35e8625b063044f6ddf74907269e07e9f14nothex", Service{})
	server.AddService("abcdefabcdef", Service{})

	inputs := map[string]string{
		"416":          "416",
		"41626":        "416261e74515b7dd1dbd55f35e8625b063044f6ddf74907269e07e9f142bc0df",
		"416261e74515": "416261e74515b7dd1dbd55f35e8625b063044f6ddf74907269e07e9f142bc0df",
		"31626":        "31626",
		"abcde":        "abcde",
		"foobar":       "foobar",
	}

	for input, expected := range inputs {
		if actual := server.getExpandedID(input); actual != expected {
			t.Error(input, "Expected:", expected, "Got:", actual)
		}
	}

}

func TestIsPrefixQuery(t *testing.T) {
	tests := []struct {
		query, name string
		expected    bool
	}{
		{"foo.bar.baz", "foo.bar.baz", true},
		{"quu.foo.bar.baz", "foo.bar.baz", true},
		{"*.bar.baz", "foo.bar.baz", true},
		{"quu.*.bar.baz", "foo.bar.baz", true},
		{"faa.foo.baz", "foo.bar.baz", false},
	}

	for _, input := range tests {
		if isPrefixQuery(strings.Split(input.query, "."), strings.Split(input.name, ".")) != input.expected {
			t.Error("Expected", input.query, "to be a valid query for", input.name)
		}
	}
}
