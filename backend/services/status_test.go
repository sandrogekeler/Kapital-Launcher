package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"kapital/backend/models"
)

func fakeStatus(results map[string]PingResult) *StatusService {
	return &StatusService{
		latest: map[string]models.ServerStatus{},
		now:    func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) },
		ping: func(_ context.Context, address string) (PingResult, error) {
			r, ok := results[address]
			if !ok {
				return PingResult{}, errors.New("connection refused")
			}
			return r, nil
		},
	}
}

func TestCheckFoldsAPingIntoAStatus(t *testing.T) {
	svc := fakeStatus(map[string]PingResult{
		"up.example:25565": {Online: true, Players: 2, MaxPlayers: 10, Version: "Paper 1.20.6", MOTD: "hi", Latency: 42 * time.Millisecond},
	})
	up := models.Chapter{ID: "lichdenstein", Server: &models.Server{Address: "up.example:25565"}}
	down := models.Chapter{ID: "frangfurd", Server: &models.Server{Address: "down.example"}}
	none := models.Chapter{ID: "luxemburg"}

	got := svc.Check(context.Background(), up)
	want := models.ServerStatus{ChapterID: "lichdenstein", Checked: true, Online: true, Players: 2, Max: 10,
		Version: "Paper 1.20.6", MOTD: "hi", LatencyMs: 42, CheckedAt: "2026-09-28T12:00:00Z"}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if got := svc.Check(context.Background(), down); got.Online || !got.Checked || got.ChapterID != "frangfurd" {
		t.Fatalf("an unreachable server is checked and offline: %+v", got)
	}
	if got := svc.Check(context.Background(), none); got.Online || !got.Checked {
		t.Fatalf("a chapter without a server is checked and offline: %+v", got)
	}
	if svc.Latest("lichdenstein") != want {
		t.Fatal("Latest must return the stored result")
	}
	if l := svc.Latest("atlantis"); l.Checked || l.ChapterID != "atlantis" {
		t.Fatalf("an unknown chapter is unchecked: %+v", l)
	}
}

func TestRunPingsOnlyServerChaptersAndStopsOnCancel(t *testing.T) {
	svc := fakeStatus(map[string]PingResult{"up.example": {Online: true}})
	chapters := []models.Chapter{
		{ID: "luxemburg"},
		{ID: "lichdenstein", Server: &models.Server{Address: "up.example"}},
		{ID: "frangfurd", Server: &models.Server{Address: "down.example"}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	var got []models.ServerStatus
	done := make(chan struct{})
	go func() {
		svc.Run(ctx, chapters, func(s models.ServerStatus) {
			got = append(got, s)
			if len(got) == 2 {
				cancel()
			}
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
	if len(got) != 2 || got[0].ChapterID != "lichdenstein" || !got[0].Online || got[1].ChapterID != "frangfurd" || got[1].Online {
		t.Fatalf("%+v", got)
	}
}
