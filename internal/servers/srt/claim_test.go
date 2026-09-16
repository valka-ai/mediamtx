package srt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/mediamtx/internal/conf"
)

type claimRoundTripper func(*http.Request) (*http.Response, error)

func (f claimRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func setClaimTransport(t *testing.T, transport claimRoundTripper) {
	t.Helper()

	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
}

func TestClaimPublisher(t *testing.T) {
	for _, status := range []int{200, 204, 299, 300, 400, 409, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			const address = "http://127.0.0.1:9102/claim"
			var requests []claimRequest
			setClaimTransport(t, func(r *http.Request) (*http.Response, error) {
				defer r.Body.Close()
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, address, r.URL.String())
				require.Equal(t, "application/json", r.Header.Get("Content-Type"))
				require.NoError(t, r.Context().Err())
				var req claimRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				requests = append(requests, req)
				return &http.Response{
					StatusCode: status,
					Body:       io.NopCloser(strings.NewReader("private response body")),
					Header:     make(http.Header),
				}, nil
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			release, err := claimPublisher(ctx, address, conf.Duration(time.Second),
				"mypath", "token=secret", "connection-id")
			require.Equal(t, []claimRequest{{
				Action: "claim",
				Path:   "mypath",
				ConnID: "connection-id",
				Query:  "token=secret",
			}}, requests)

			if status >= 300 {
				require.Nil(t, release)
				require.EqualError(t, err, fmt.Sprintf("publisher claim refused with status %d", status))
				require.NotContains(t, err.Error(), "token=secret")
				require.NotContains(t, err.Error(), "private response body")
				require.NotContains(t, err.Error(), address)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, release)
			cancel()
			release()
			require.Len(t, requests, 2)
			require.Equal(t, claimRequest{
				Action: "release",
				Path:   "mypath",
				ConnID: "connection-id",
				Query:  "token=secret",
			}, requests[1])
		})
	}
}

func TestClaimPublisherTransportError(t *testing.T) {
	setClaimTransport(t, func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		return nil, fmt.Errorf("transport failure for %s", r.URL)
	})

	for _, address := range []string{"", "://invalid?secret", "http://127.0.0.1:9102/claim?endpoint=secret"} {
		release, err := claimPublisher(context.Background(), address, conf.Duration(time.Second),
			"mypath", "token=secret", "connection-id")
		require.Error(t, err)
		require.Nil(t, release)
		require.NotContains(t, err.Error(), "secret")
		require.NotContains(t, err.Error(), "127.0.0.1")
	}
}

func TestClaimPublisherTimeout(t *testing.T) {
	setClaimTransport(t, func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		<-r.Context().Done()
		return nil, r.Context().Err()
	})

	start := time.Now()
	release, err := claimPublisher(context.Background(), "http://127.0.0.1:9102/claim",
		conf.Duration(50*time.Millisecond), "mypath", "token=secret", "connection-id")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, release)
	require.Less(t, time.Since(start), time.Second)
	require.NotContains(t, err.Error(), "token=secret")
	require.NotContains(t, err.Error(), "127.0.0.1")
}

func TestClaimPublisherRedirect(t *testing.T) {
	const address = "http://127.0.0.1:9102/claim"
	redirected := false
	setClaimTransport(t, func(r *http.Request) (*http.Response, error) {
		if r.Body != nil {
			defer r.Body.Close()
		}
		res := &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": {"http://127.0.0.1:9103/claim"}},
			Body:       io.NopCloser(strings.NewReader("private redirect body")),
		}
		if r.URL.String() != address {
			redirected = true
			res.StatusCode = http.StatusOK
		}
		return res, nil
	})

	release, err := claimPublisher(context.Background(), address, conf.Duration(time.Second),
		"mypath", "token=secret", "connection-id")
	require.EqualError(t, err, "publisher claim refused with status 302")
	require.Nil(t, release)
	require.False(t, redirected)
}
