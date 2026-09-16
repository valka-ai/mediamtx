package srt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bluenviron/mediamtx/internal/conf"
)

// claimRequest is the body sent to the fleet-wide publisher claim endpoint.
type claimRequest struct {
	Action string `json:"action"` // "claim" | "release"
	Path   string `json:"path"`
	ConnID string `json:"conn_id"`
	Query  string `json:"query,omitempty"`
}

// claimPublisher asks an external authority whether this instance may become the
// confirmed source of path. It runs on the per-connection goroutine, never on the
// path manager's, so a slow or hung authority cannot stall admission for other
// paths on this instance.
//
// A non-2xx answer, a transport error or a timeout all deny the claim: the
// authority is the only thing that can say yes.
//
// It returns a release function, which is best-effort and must be called when the
// publisher goes away.
func claimPublisher(
	ctx context.Context,
	address string,
	timeout conf.Duration,
	path string,
	query string,
	connID string,
) (func(), error) {
	post := func(ctx context.Context, action string) error {
		body, err := json.Marshal(&claimRequest{
			Action: action,
			Path:   path,
			ConnID: connID,
			Query:  query,
		})
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("invalid publisher claim request")
		}
		req.Header.Set("Content-Type", "application/json")

		res, err := (&http.Client{
			Timeout: time.Duration(timeout),
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}).Do(req)
		if err != nil {
			// Transport errors can include the endpoint URL. Keep it out of logs.
			if ctx.Err() != nil {
				return fmt.Errorf("publisher claim request failed: %w", ctx.Err())
			}
			return fmt.Errorf("publisher claim request failed")
		}
		defer res.Body.Close()

		if res.StatusCode < 200 || res.StatusCode > 299 {
			return fmt.Errorf("publisher claim refused with status %d", res.StatusCode)
		}
		return nil
	}

	claimCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout))
	defer cancel()

	if err := post(claimCtx, "claim"); err != nil {
		return nil, err
	}

	return func() {
		// Deliberately detached from ctx: the connection's context is already
		// cancelled by the time a publisher is torn down, and a release that
		// cannot be sent leaves the claim to the authority's own expiry.
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), time.Duration(timeout))
		defer releaseCancel()
		_ = post(releaseCtx, "release")
	}, nil
}
