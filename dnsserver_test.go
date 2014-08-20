package main

import (
	"github.com/miekg/dns"
	"testing"
	"time"
)

const TEST_ADDR = "127.0.0.1:9953"

func TestDNSResponse(t *testing.T) {
	server := NewDNSServer(&Config{
		dnsAddr: TEST_ADDR,
	})
	go server.Start()

	// Allow some time for server to start
	time.Sleep(150 * time.Millisecond)

	m := new(dns.Msg)
	m.Id = dns.Id()
	m.Question = []dns.Question{
		dns.Question{"docker.", dns.TypeA, dns.ClassINET},
	}
	c := new(dns.Client)
	_, _, err := c.Exchange(m, TEST_ADDR)

	if err != nil {
		t.Error("Error response from the server", err)
	}

	server.Stop()

	c = new(dns.Client)
	_, _, err = c.Exchange(m, TEST_ADDR)

	if err == nil {
		t.Error("Server still running but should be shut down.")
	}
}
