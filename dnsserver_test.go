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
	time.Sleep(150 * time.Millisecond)

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

	server.Stop()

	c = new(dns.Client)
	_, _, err = c.Exchange(m, TEST_ADDR)

	if err == nil {
		t.Error("Server still running but should be shut down.")
	}
}
