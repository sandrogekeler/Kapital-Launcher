package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// A chapter's instance syncs its pack on every Play through packwiz-installer
// (ADR-3). Both jars are MIT-licensed, fetched by the launcher from their
// GitHub releases, pinned here by size and SHA-256, and copied into each
// instance. The bootstrap then runs with --bootstrap-no-update: left to
// itself it asks api.github.com for a newer installer on every launch, which
// an unauthenticated client may do 60 times an hour per address.
//
// To move to a newer release, download both, read their sizes and digests,
// and update this list in the same commit as the reason.

type pinnedJar struct {
	name   string
	url    string
	size   int64
	sha256 string
}

var packwizJars = []pinnedJar{
	{
		name:   "packwiz-installer-bootstrap.jar",
		url:    "https://github.com/packwiz/packwiz-installer-bootstrap/releases/download/v0.0.3/packwiz-installer-bootstrap.jar",
		size:   98989,
		sha256: "a8fbb24dc604278e97f4688e82d3d91a318b98efc08d5dbfcbcbcab6443d116c",
	},
	{
		name:   "packwiz-installer.jar",
		url:    "https://github.com/packwiz/packwiz-installer/releases/download/v0.5.14/packwiz-installer.jar",
		size:   4378828,
		sha256: "c9f646908d340d84773948a9a7d98bc1dae250d35e1016dc6e2b8459760b5598",
	},
}

const jarDownloadTimeout = 2 * time.Minute

// packwizJarCache keeps the pinned jars under the launcher's data dir, so a
// second chapter's install does not download them again.
type packwizJarCache struct {
	dir          string
	jars         []pinnedJar
	client       *http.Client
	allowedHosts []string
}

func newPackwizJarCache(dataDir string) *packwizJarCache {
	c := &packwizJarCache{
		dir:          filepath.Join(dataDir, "packwiz"),
		jars:         packwizJars,
		allowedHosts: prismDownloadHosts,
	}
	c.client = &http.Client{CheckRedirect: c.checkRedirect}
	return c
}

// ensure returns the path of every pinned jar, downloading any that is
// missing or no longer matches its digest.
func (c *packwizJarCache) ensure(ctx context.Context) (map[string]string, error) {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return nil, err
	}
	paths := map[string]string{}
	for _, jar := range c.jars {
		path := filepath.Join(c.dir, jar.name)
		if err := matchesPin(path, jar); err != nil {
			if err := c.download(ctx, jar, path); err != nil {
				return nil, err
			}
		}
		paths[jar.name] = path
	}
	return paths, nil
}

func (c *packwizJarCache) download(ctx context.Context, jar pinnedJar, path string) error {
	ctx, cancel := context.WithTimeout(ctx, jarDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jar.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", prismUserAgent)
	if err := c.checkHost(req.URL); err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", jar.name, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", jar.name, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, jar.size+1))
	if err != nil {
		return fmt.Errorf("download %s: %w", jar.name, err)
	}
	if err := checkPin(data, jar); err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func (c *packwizJarCache) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	return c.checkHost(req.URL)
}

func (c *packwizJarCache) checkHost(u *url.URL) error {
	if u.Scheme != "https" || !slices.Contains(c.allowedHosts, u.Hostname()) {
		return fmt.Errorf("refusing to fetch packwiz from %s", u.Redacted())
	}
	return nil
}

func matchesPin(path string, jar pinnedJar) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return checkPin(data, jar)
}

func checkPin(data []byte, jar pinnedJar) error {
	if int64(len(data)) != jar.size {
		return fmt.Errorf("%s is %d bytes, pinned at %d", jar.name, len(data), jar.size)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != jar.sha256 {
		return fmt.Errorf("%s does not match its pinned digest (got %s)", jar.name, got)
	}
	return nil
}
