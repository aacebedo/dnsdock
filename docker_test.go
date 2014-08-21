package main

import (
	"testing"
)

func TestGetImageName(t *testing.T) {
	inputs := map[string]string{
		"foo":                                  "foo",
		"foo:latest":                           "foo",
		"tonistiigi/foo":                       "foo",
		"tonistiigi/foo-bar:v1.0":              "foo-bar",
		"domain.com/tonistiigi/bar.baz:latest": "bar.baz",
	}

	for input, expected := range inputs {
		t.Log(input)
		if actual := getImageName(input); actual != expected {
			t.Error(input, "Expected:", expected, "Got:", actual)
		}
	}
}

func TestImageNameIsSHA(t *testing.T) {
	inputs := []struct {
		name, SHA string
		expected  bool
	}{
		{"abc", "abcdef", false},
		{"abcdef", "abcdef123", true},
		{"foobar", "foobar", false},
		{"abcdef", "12345678", false},
	}

	for _, input := range inputs {
		t.Log(input.SHA)
		if actual := imageNameIsSHA(input.name, input.SHA); actual != input.expected {
			t.Error(input.name, input.SHA, "Expected:", input.expected, "Got:", actual)
		}
	}
}

func TestCleanContainerName(t *testing.T) {
	inputs := map[string]string{
		"foo":  "foo",
		"/foo": "foo",
	}

	for input, expected := range inputs {
		t.Log(input)
		if actual := cleanContainerName(input); actual != expected {
			t.Error(input, "Expected:", expected, "Got:", actual)
		}
	}
}
