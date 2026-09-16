package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/servers/srt"
	"github.com/bluenviron/mediamtx/internal/test"
)

func TestPublisherClaimReload(t *testing.T) {
	for _, ca := range []struct {
		name   string
		change func(*conf.Conf)
		close  bool
	}{
		{"address", func(c *conf.Conf) { c.PublisherClaimHTTPAddress = "http://127.0.0.1:9103/claim" }, true},
		{"timeout", func(c *conf.Conf) { c.PublisherClaimTimeout = conf.Duration(time.Second) }, true},
		{"unrelated", func(c *conf.Conf) { c.APIAddress = "127.0.0.1:9998" }, false},
		{"unchanged", func(_ *conf.Conf) {}, false},
	} {
		t.Run(ca.name, func(t *testing.T) {
			currentConf := &conf.Conf{
				SRT:                       true,
				PublisherClaimHTTPAddress: "http://127.0.0.1:9102/claim",
				PublisherClaimTimeout:     conf.Duration(2 * time.Second),
			}
			server := &srt.Server{
				Address:                   "127.0.0.1:0",
				ReadTimeout:               conf.Duration(time.Second),
				WriteTimeout:              conf.Duration(time.Second),
				UDPMaxPayloadSize:         1472,
				PublisherClaimHTTPAddress: currentConf.PublisherClaimHTTPAddress,
				PublisherClaimTimeout:     currentConf.PublisherClaimTimeout,
				Parent:                    test.NilLogger,
			}
			require.NoError(t, server.Initialize())
			p := &Core{srtServer: server}
			p.conf.Store(currentConf)
			defer p.closeResources(nil)

			newConf := currentConf.Clone()
			ca.change(newConf)
			p.closeResources(newConf)
			if ca.close {
				require.Nil(t, p.srtServer)
			} else {
				require.Same(t, server, p.srtServer)
			}
		})
	}
}
