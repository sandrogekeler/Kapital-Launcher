package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"kapital/backend/models"
)

// packwizStateName is packwiz-installer's own record in the game folder:
// the hashes of the pack.toml and index.toml it last synced and every file
// it placed (#71). The launcher reads one value from it, the pack.toml
// hash; the file list is dropped unread.
const (
	packwizStateName  = "packwiz.json"
	maxPackwizState   = 8 << 20
	packwizHashSHA256 = "sha256"
)

// PackState compares the pack a chapter's instance last synced with what its
// source serves now: the pack URL the instance syncs from when the launcher
// wrote it, else the developer override, else the manifest's. A chapter
// whose instance is absent, or has never synced, is reported as not
// installed; a source that cannot be read leaves the state unchecked. Only
// the manifest's rules or the loopback rule let a URL be fetched, as at
// Install.
func (c *InstanceCreator) PackState(ctx context.Context, chapter models.Chapter, report models.InstanceReport, override string) models.PackState {
	state := models.PackState{ChapterID: chapter.ID}
	if !report.Present[chapter.ID] || report.Dir == "" {
		return state
	}
	installedHash, ok := installedPackHash(filepath.Join(report.Dir, chapter.Instance.ID))
	if !ok {
		return state
	}
	state.Installed = true

	packURL, check := c.packSource(chapter, report, override)
	if packURL == "" {
		return state
	}
	raw, err := c.fetchPackTOML(ctx, packURL, check)
	if err != nil {
		slog.Warn("pack state", "chapter", chapter.ID, "error", err)
		return state
	}
	sum := sha256.Sum256(raw)
	state.Checked = true
	state.UpToDate = hex.EncodeToString(sum[:]) == installedHash
	if v, err := scanPackVersions(bytes.NewReader(raw)); err == nil {
		state.Version = v.pack
	}
	return state
}

// packSource is the pack.toml the instance syncs from and the rule its URL
// must pass: the URL read back from the instance's pre-launch command when
// the launcher wrote it, else the developer override, else the manifest's.
func (c *InstanceCreator) packSource(chapter models.Chapter, report models.InstanceReport, override string) (string, func(field, raw string) error) {
	local := func(_, raw string) error { return CheckLocalPackURL(raw) }
	if url := report.PackURL[chapter.ID]; url != "" {
		if IsLocalPackURL(url) {
			return url, local
		}
		if c.checkPackURL(chapter.ID+".pack.packwiz", url) == nil {
			return url, c.checkPackURL
		}
		return "", nil
	}
	if override != "" && IsLocalPackURL(override) {
		return override, local
	}
	if chapter.Pack.Packwiz != nil && c.checkPackURL(chapter.ID+".pack.packwiz", *chapter.Pack.Packwiz) == nil {
		return *chapter.Pack.Packwiz, c.checkPackURL
	}
	return "", nil
}

// installedPackHash reads the pack.toml hash packwiz-installer recorded in
// the instance's game folder ("minecraft", or ".minecraft" in an older
// instance). No record, or one not in SHA-256, means the pack has not been
// synced in a way the launcher can compare.
func installedPackHash(instanceDir string) (string, bool) {
	for _, game := range []string{"minecraft", ".minecraft"} {
		f, err := os.Open(filepath.Join(instanceDir, game, packwizStateName))
		if err != nil {
			continue
		}
		var doc struct {
			PackFileHash struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"packFileHash"`
		}
		err = json.NewDecoder(io.LimitReader(f, maxPackwizState)).Decode(&doc)
		f.Close() //nolint:errcheck // read-only file, nothing to flush
		if err != nil || doc.PackFileHash.Type != packwizHashSHA256 || doc.PackFileHash.Value == "" {
			return "", false
		}
		return doc.PackFileHash.Value, true
	}
	return "", false
}
