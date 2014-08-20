package main

import (
	"github.com/miekg/dns"
	"testing"
	"time"
)

const TEST_ADDR = "127.0.0.1:9953"

func TestDNSResponse(t *testing.T) {
	config := NewConfig()
	config.dnsAddr = TEST_ADDR

	server := NewDNSServer(config)
	go server.Start()

	// Allow some time for server to start
	time.Sleep(250 * time.Millisecond)

	m := new(dns.Msg)
	m.Id = dns.Id()
	m.RecursionDesired = true
	m.Question = []dns.Question{
		dns.Question{"google.com.", dns.TypeA, dns.ClassINET},
	}
	c := new(dns.Client)
	in, _, err := c.Exchange(m, TEST_ADDR)

	if err != nil {
		t.Error("Error response from the server", err)
	}

	if len(in.Answer) < 3 {
		t.Error("DNS request only responded ", len(in.Answer), "answers")
	}

	// // This test is slow and pointless
	// server.Stop()
	//
	// c = new(dns.Client)
	// _, _, err = c.Exchange(m, TEST_ADDR)
	//
	// if err == nil {
	// 	t.Error("Server still running but should be shut down.")
	// }
}

func TestServiceManagement(t *testing.T) {
	list := ServiceListProvider(NewDNSServer(NewConfig()))

	if len(list.GetAllServices()) != 0 {
		t.Error("Initial service count should be 0.")
	}

	A := Service{Name: "bar"}
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

	list.AddService("boo", Service{Name: "boo"})

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

}
