package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"kapital/backend/models"
)

// A chapter's map is a BlueMap web page the launcher frames (issue 161). An
// iframe cannot tell an unreachable server from a slow one, so Go asks first:
// one short GET, and the page shows the frame only when something answered.
// The address is the manifest's own, held to checkMapURL (SECURITY_CHECKLIST
// S5.4); what comes back is a status and nothing else is read.
const (
	// mapCheckTimeout bounds the whole request, the connection included, as
	// the server ping's timeout does.
	mapCheckTimeout = 5 * time.Second
	// maxMapBody is how much of a response is read before it is dropped: enough
	// for the connection to close cleanly, never the map itself.
	maxMapBody = 4 << 10
)

// CheckMap says whether a chapter's map answers at raw, an address of the
// manifest's. Any HTTP status counts as reachable: BlueMap answers 200, and a
// 404 or a redirect still means a web server is there. An empty address is a
// chapter with no map, and an address that fails the manifest rule is refused
// here as well, whatever the caller passed. A failure is a status, never an
// error, so the page can show it and offer to try again.
func CheckMap(ctx context.Context, raw string) models.MapStatus {
	if raw == "" {
		return models.MapStatus{Reason: "no map"}
	}
	if err := checkMapURL("map", raw); err != nil {
		slog.Warn("map check refused", "error", err)
		return models.MapStatus{URL: raw, Reason: "not a valid map address"}
	}
	return probeMap(ctx, newMapClient(mapCheckTimeout), raw)
}

// newMapClient is a client that follows no redirect, so a map address that
// sends the request elsewhere is only ever told apart by its status.
func newMapClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// probeMap is CheckMap's request, with the client and without the address
// rule, so a test can point it at a local server.
func probeMap(ctx context.Context, client *http.Client, raw string) models.MapStatus {
	status := models.MapStatus{URL: raw}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		status.Reason = "no answer"
		return status
	}
	req.Header.Set("User-Agent", prismUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		status.Reason = "no answer"
		var netErr net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
			status.Reason = "timed out"
		}
		// The error's text is left out of the log, as the ping's is.
		slog.Info("map check", "reachable", false, "reason", status.Reason)
		return status
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxMapBody)); err != nil {
		// The status arrived, which is the answer; a body that breaks off is not.
		slog.Debug("map check: body", "error", err)
	}
	status.Reachable = true
	slog.Info("map check", "reachable", true, "status", resp.StatusCode)
	return status
}
