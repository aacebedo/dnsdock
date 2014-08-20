package main

import (
	"testing"
)

func TestDomainCreation(t *testing.T) {
	var tests = map[string]string{
		"foo":           "foo",
		"foo.":          "foo",
		".foo.docker.":  "foo.docker",
		".foo..docker.": "foo.docker",
		"foo.docker..":  "foo.docker",
	}

	for input, expected := range tests {
		t.Log(input)
		d := NewDomain(input)
		if actual := d.String(); actual != expected {
			t.Error(input, "Expected:", expected, "Got:", actual)
		}
	}
}
