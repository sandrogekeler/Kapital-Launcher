package services

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A test server stands in for the map. Nothing here dials the real address.

func TestProbeMapCountsAnyStatusAsReachable(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusNotFound, http.StatusInternalServerError, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		got := probeMap(t.Context(), newMapClient(time.Second), srv.URL)
		srv.Close()
		if !got.Reachable || got.URL != srv.URL || got.Reason != "" {
			t.Errorf("status %d: %+v", code, got)
		}
	}
}

func TestProbeMapDropsTheBodyAfterAFewKiB(t *testing.T) {
	var sent atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunk := []byte(strings.Repeat("x", 1024))
		for range 128 * 1024 {
			n, err := w.Write(chunk)
			sent.Add(int64(n))
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()
	got := probeMap(t.Context(), newMapClient(2*time.Second), srv.URL)
	if !got.Reachable {
		t.Fatalf("%+v", got)
	}
	// 128 MiB were on offer; the client read at most maxMapBody of them (the
	// rest is whatever the transport had already buffered).
	if sent.Load() >= 32<<20 {
		t.Errorf("the whole body was read: %d bytes", sent.Load())
	}
}

func TestProbeMapReportsARefusedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	url := "http://" + ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	got := probeMap(t.Context(), newMapClient(time.Second), url)
	if got.Reachable || got.Reason == "" || got.URL != url {
		t.Fatalf("%+v", got)
	}
}

func TestProbeMapTimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	start := time.Now()
	got := probeMap(t.Context(), newMapClient(80*time.Millisecond), srv.URL)
	if got.Reachable || got.Reason != "timed out" {
		t.Fatalf("%+v", got)
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("took %v", time.Since(start))
	}
}

func TestProbeMapStopsWhenTheCallerGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(50*time.Millisecond, cancel)
	got := probeMap(ctx, newMapClient(5*time.Second), srv.URL)
	if got.Reachable || got.Reason == "" {
		t.Fatalf("%+v", got)
	}
}

func TestProbeMapDoesNotFollowARedirect(t *testing.T) {
	var elsewhere atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhere.Add(1)
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusFound)
	}))
	defer srv.Close()
	got := probeMap(t.Context(), newMapClient(time.Second), srv.URL)
	// The redirect is an answer: a web server is there. Where it points is
	// never asked.
	if !got.Reachable {
		t.Fatalf("%+v", got)
	}
	if elsewhere.Load() != 0 {
		t.Fatal("the redirect was followed")
	}
}

func TestProbeMapIdentifiesTheLauncherAndSendsAGet(t *testing.T) {
	var method, agent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, agent = r.Method, r.Header.Get("User-Agent")
	}))
	defer srv.Close()
	probeMap(t.Context(), newMapClient(time.Second), srv.URL)
	if method != http.MethodGet || agent != prismUserAgent {
		t.Fatalf("%s %q", method, agent)
	}
}

// CheckMap holds the address to the manifest's rule itself, so a caller that
// reaches it with anything else does not make a request.
func TestCheckMapAsksOnlyAboutAManifestShapedAddress(t *testing.T) {
	var hit atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.Add(1)
	}))
	defer srv.Close()

	if got := CheckMap(t.Context(), ""); got.Reachable || got.URL != "" || got.Reason != "no map" {
		t.Errorf("no map: %+v", got)
	}
	for _, raw := range []string{srv.URL, "http://127.0.0.1:1", "http://spiral-reminders.tun.ply.gg", "file:///C:/Windows", "http://a.tun.ply.gg:1111/x?y"} {
		got := CheckMap(t.Context(), raw)
		if got.Reachable || got.Reason == "" {
			t.Errorf("%s: %+v", raw, got)
		}
	}
	if hit.Load() != 0 {
		t.Fatal("a refused address was requested")
	}
}
