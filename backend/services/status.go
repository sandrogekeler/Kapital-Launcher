package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"kapital/backend/models"
)

// EventServerStatus is the Wails event carrying a models.ServerStatus. Every
// payload names its chapter, and a listener filters on it.
const EventServerStatus = "server:status"

// StatusInterval is how often each server is pinged while the app is open.
// The vanilla client's server list refreshes on demand; once a minute is
// enough to keep a "Server online" line honest without being a nuisance.
const StatusInterval = time.Minute

// StatusService pings every chapter that has a server, on a ticker and on
// demand, and hands each result to an emitter. It only ever asks the
// addresses the validated manifest names, the one each chapter's saved choice
// picks (ServerAddress).
type StatusService struct {
	ping func(ctx context.Context, address string) (PingResult, error)
	now  func() time.Time

	mu     sync.Mutex
	latest map[string]models.ServerStatus
}

// NewStatusService uses the real Ping.
func NewStatusService() *StatusService {
	return &StatusService{ping: Ping, now: time.Now, latest: map[string]models.ServerStatus{}}
}

// NewStatusServiceWithPing is a StatusService that asks ping instead of the
// network, for tests of what the callers dial.
func NewStatusServiceWithPing(ping func(ctx context.Context, address string) (PingResult, error)) *StatusService {
	return &StatusService{ping: ping, now: time.Now, latest: map[string]models.ServerStatus{}}
}

// Check pings one chapter's server now and returns the result, also storing
// it for Latest. The address is the chapter's chosen one (ServerAddress);
// "" is a chapter with no server, which is not dialled. An unreachable server
// is a status with Online false, not an error: the UI's question is "is it
// up", and "no" is an answer.
func (s *StatusService) Check(ctx context.Context, chapter models.Chapter, address string) models.ServerStatus {
	status := models.ServerStatus{ChapterID: chapter.ID, Checked: true}
	if address != "" {
		result, err := s.ping(ctx, address)
		if err != nil {
			slog.Info("server status", "chapter", chapter.ID, "online", false, "error", err)
		} else {
			status.Online = true
			status.Players = result.Players
			status.Max = result.MaxPlayers
			status.Version = result.Version
			status.MOTD = result.MOTD
			status.LatencyMs = result.Latency.Milliseconds()
		}
	}
	status.CheckedAt = s.now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	s.latest[chapter.ID] = status
	s.mu.Unlock()
	return status
}

// Latest returns the last result for a chapter, or an unchecked status.
func (s *StatusService) Latest(chapterID string) models.ServerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.latest[chapterID]; ok {
		return st
	}
	return models.ServerStatus{ChapterID: chapterID}
}

// Run pings every server chapter immediately and then every StatusInterval,
// handing each result to emit, until ctx is cancelled. Chapters without a
// server are skipped; they have nothing to report. address says where a
// chapter is pinged, asked again on every tick so a changed choice is followed
// without a restart.
func (s *StatusService) Run(ctx context.Context, chapters []models.Chapter, address func(models.Chapter) string, emit func(models.ServerStatus)) {
	tick := func() {
		for _, c := range chapters {
			if c.Server == nil {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			// A choice changed while the ping was out (issue 151) makes its
			// answer stale: the save pings the new address itself, so this one
			// is dropped rather than sent after it.
			dialed := address(c)
			status := s.Check(ctx, c, dialed)
			if address(c) != dialed {
				continue
			}
			emit(status)
		}
	}
	tick()
	ticker := time.NewTicker(StatusInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}
