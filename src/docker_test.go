package main

import (
	"reflect"
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

func TestSplitEnv(t *testing.T) {
	input := []string{"FOO=something ", "BAR_BAZ=dsfjds sadf asd"}
	expected := map[string]string{
		"FOO":     "something",
		"BAR_BAZ": "dsfjds sadf asd",
	}
	actual := splitEnv(input)
	if eq := reflect.DeepEqual(actual, expected); !eq {
		t.Error(input, "Expected:", expected, "Got:", actual)
	}
}

func TestOverrideFromEnv(t *testing.T) {
	getService := func() *Service {
		service := NewService()
		service.Name = "myfoo"
		service.Image = "mybar"
		return service
	}

	s := getService()
	s = overrideFromEnv(s, map[string]string{"SERVICE_IGNORE": "1"})
	if s != nil {
		t.Error("Skipping failed")
	}

	s = getService()
	s = overrideFromEnv(s, map[string]string{"DNSDOCK_IGNORE": "1"})
	if s != nil {
		t.Error("Skipping failed(2)")
	}

	s = getService()
	s = overrideFromEnv(s, map[string]string{"DNSDOCK_NAME": "master", "DNSDOCK_IMAGE": "mysql", "DNSDOCK_TTL": "22"})
	if s.Name != "master" || s.Image != "mysql" || s.Ttl != 22 {
		t.Error("Invalid DNSDOCK override", s)
	}

	s = getService()
	s = overrideFromEnv(s, map[string]string{"SERVICE_TAGS": "master,something", "SERVICE_NAME": "mysql", "SERVICE_REGION": "us2"})
	if s.Name != "master" || s.Image != "mysql.us2" {
		t.Error("Invalid SERVICE overrid", s)
	}

}
