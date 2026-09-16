package conf

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPublisherClaimConfig(t *testing.T) {
	for _, ca := range []struct {
		name    string
		address string
		err     string
	}{
		{"empty", "", "'publisherClaimHTTPAddress' must be set when srt is enabled"},
		{"https", "https://127.0.0.1:9102/claim", "'publisherClaimHTTPAddress' must be an http URL"},
		{"other scheme", "ftp://127.0.0.1:9102/claim", "'publisherClaimHTTPAddress' must be an http URL"},
		{"relative", "/claim", "'publisherClaimHTTPAddress' must be an http URL"},
		{"malformed", "http://[::1/claim", "'publisherClaimHTTPAddress' must be an http URL"},
		{"remote", "http://192.0.2.1:9102/claim", "'publisherClaimHTTPAddress' must have a loopback host"},
		{"lookalike host", "http://localhost.example:9102/claim", "'publisherClaimHTTPAddress' must have a loopback host"},
		{"unspecified host", "http://0.0.0.0:9102/claim", "'publisherClaimHTTPAddress' must have a loopback host"},
		{"userinfo", "http://user:secret@127.0.0.1:9102/claim",
			"'publisherClaimHTTPAddress' must not contain userinfo or a fragment"},
		{"empty userinfo", "http://@127.0.0.1:9102/claim",
			"'publisherClaimHTTPAddress' must not contain userinfo or a fragment"},
		{"fragment", "http://127.0.0.1:9102/claim#secret",
			"'publisherClaimHTTPAddress' must not contain userinfo or a fragment"},
		{"empty fragment", "http://127.0.0.1:9102/claim#",
			"'publisherClaimHTTPAddress' must not contain userinfo or a fragment"},
		{"missing port", "http://127.0.0.1/claim", "'publisherClaimHTTPAddress' must have a valid port"},
		{"empty port", "http://127.0.0.1:/claim", "'publisherClaimHTTPAddress' must have a valid port"},
		{"zero port", "http://127.0.0.1:0/claim", "'publisherClaimHTTPAddress' must have a valid port"},
		{"port overflow", "http://127.0.0.1:65536/claim", "'publisherClaimHTTPAddress' must have a valid port"},
		{"negative port", "http://127.0.0.1:-1/claim", "'publisherClaimHTTPAddress' must be an http URL"},
		{"nonnumeric port", "http://127.0.0.1:http/claim", "'publisherClaimHTTPAddress' must be an http URL"},
		{"unbracketed ipv6", "http://::1:9102/claim", "'publisherClaimHTTPAddress' must be an http URL"},
		{"ipv4", "http://127.0.0.1:9102/claim", ""},
		{"ipv6", "http://[::1]:9102/claim", ""},
		{"localhost", "http://localhost:9102/claim", ""},
		{"localhost case", "http://LOCALHOST:9102/claim", ""},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var conf Conf
			conf.setDefaults()
			conf.PublisherClaimHTTPAddress = ca.address
			err := conf.Validate(nil)
			if ca.err != "" {
				require.EqualError(t, err, ca.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPublisherClaimTimeout(t *testing.T) {
	for _, timeout := range []Duration{0, -Duration(time.Second)} {
		t.Run(time.Duration(timeout).String(), func(t *testing.T) {
			var conf Conf
			conf.setDefaults()
			conf.PublisherClaimHTTPAddress = "http://127.0.0.1:9102/claim"
			conf.PublisherClaimTimeout = timeout
			require.EqualError(t, conf.Validate(nil), "'publisherClaimTimeout' must be greater than zero")
		})
	}
}

func TestPublisherClaimDefaults(t *testing.T) {
	var conf Conf
	conf.setDefaults()
	require.Equal(t, 2*Duration(time.Second), conf.PublisherClaimTimeout)
	require.Empty(t, conf.PublisherClaimHTTPAddress)
	require.EqualError(t, conf.Validate(nil), "'publisherClaimHTTPAddress' must be set when srt is enabled")

	conf.SRT = false
	require.NoError(t, conf.Validate(nil))
}
