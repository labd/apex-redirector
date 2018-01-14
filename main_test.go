package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseArgs(t *testing.T) {
	os.Args = []string{"./apex-redirector", "--secret", "geheim"}
	result := parseArgs()

	expected := &options{
		bindHTTP:  "127.0.0.1:8080",
		bindHTTPS: "127.0.0.1:8443",
		secret:    "geheim",
	}
	assert.Equal(t, expected, result)
}
