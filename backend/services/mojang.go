package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"kapital/backend/models"
)

// The player's skin face and UUID, from Mojang's public profile endpoints, no
// sign-in and no token (ADR-13, issue 193). Three hosts are asked, in turn:
//
//	api.mojang.com/users/profiles/minecraft/<name>   the UUID and canonical name
//	sessionserver.mojang.com/session/minecraft/profile/<uuid>   the textures property
//	textures.minecraft.net/texture/<hash>   the skin PNG
//
// Only the profile name leaves the machine, in the first request; the second
// carries the UUID the first returned. What is kept, in a folder of its own
// (mojang/) in the app data dir: the UUID, the canonical name and when they
// were asked (lookup.json), and the 8x8 face cut from the skin, scaled up and
// stored as <uuid>.png. The skin itself is decoded in memory and dropped. The
// page loads the face at /mojang-face/<uuid>.png (FaceMiddleware), so the CSP
// is unchanged. A name Mojang does not know, or Mojang being out of reach, is a
// status and never an error: the page falls back to the initials.

const (
	mojangDir       = "mojang"
	mojangLookup    = "lookup.json"
	mojangFaceRoute = "/mojang-face/"
	// mojangTTL is how long a lookup and its face are used without asking again.
	mojangTTL = 24 * time.Hour
	// mojangTimeout bounds each request, the connection included.
	mojangTimeout = 8 * time.Second

	mojangAPIBase      = "https://api.mojang.com"
	mojangSessionBase  = "https://sessionserver.mojang.com"
	mojangTexturesBase = "https://textures.minecraft.net"
	mojangTexturesHost = "textures.minecraft.net"

	// Bounds on what is read (S4.3). A profile answer is under 300 bytes, a
	// session profile a little over 1 KiB, a skin 1 to 10 KiB.
	maxMojangLookup  = 4 << 10
	maxMojangSession = 32 << 10
	maxMojangSkin    = 256 << 10

	faceScale = 8
)

var (
	// minecraftName is Minecraft's own rule for a Java Edition name, checked
	// before any request and held by an offline name too (issue 192).
	minecraftName = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)
	// mojangID is a profile id, undashed.
	mojangID = regexp.MustCompile(`^[0-9a-f]{32}$`)
	// skinPath is a texture's path on textures.minecraft.net: a hex hash.
	skinPath = regexp.MustCompile(`^/texture/([0-9a-f]{16,64})$`)
	// faceFile is a cached face's name below the route and the folder.
	faceFile = regexp.MustCompile(`^[0-9a-f]{32}\.png$`)
)

// errMojangUnreachable is any failure to get an answer that is not "no such
// profile": no network, a timeout, a rate limit, an unexpected status or body.
var errMojangUnreachable = errors.New("mojang: no usable answer")

// mojangEntry is the one lookup kept: who Mojang said the name was, and when.
type mojangEntry struct {
	// Key is the name as asked, lower case: Mojang's lookup ignores case.
	Key       string    `json:"key"`
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
	CheckedAt time.Time `json:"checkedAt"`
}

// MojangService looks a player up and keeps the face.
type MojangService struct {
	dir                            string
	apiBase, sessionBase, textures string
	client                         *http.Client
	now                            func() time.Time

	// mu serialises lookups, the cache files and last.
	mu sync.Mutex
	// last is the UUID of the most recent lookup that found a profile, "" when
	// the latest lookup found none: what CopyUUID hands over.
	last string
}

// NewMojangService keeps its cache under dataDir/mojang.
func NewMojangService(dataDir string) *MojangService {
	return &MojangService{
		dir:         filepath.Join(dataDir, mojangDir),
		apiBase:     mojangAPIBase,
		sessionBase: mojangSessionBase,
		textures:    mojangTexturesBase,
		now:         time.Now,
		client: &http.Client{
			Timeout: mojangTimeout,
			// A fixed path on an allowlisted host: a redirect would be a
			// surprise, and is not followed.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// ValidMinecraftName is whether name could be a Java Edition profile name.
func ValidMinecraftName(name string) bool { return minecraftName.MatchString(name) }

// Profile looks name up. It never returns an error: every outcome is a status.
// The cached lookup is used for a day; a stale one is asked again, and kept as
// the answer when Mojang cannot be reached.
func (m *MojangService) Profile(ctx context.Context, name string) models.PlayerProfile {
	name = strings.TrimSpace(name)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.last = ""
	if !ValidMinecraftName(name) {
		return models.PlayerProfile{Status: models.PlayerInvalid}
	}
	p := m.profile(ctx, name)
	if p.Status == models.PlayerFound {
		m.last = strings.ReplaceAll(p.UUID, "-", "")
	}
	return p
}

// CopyUUID is the dashed UUID of the latest found profile, for the clipboard.
func (m *MojangService) CopyUUID() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.last == "" {
		return "", errors.New("no player profile to copy")
	}
	return dashUUID(m.last), nil
}

func (m *MojangService) profile(ctx context.Context, name string) models.PlayerProfile {
	key := strings.ToLower(name)
	entry, have := m.readEntry()
	have = have && entry.Key == key
	fresh := have && m.now().Sub(entry.CheckedAt) < mojangTTL

	if !fresh {
		id, canonical, err := m.lookup(ctx, name)
		switch {
		case errors.Is(err, errNoProfile):
			m.forget()
			return models.PlayerProfile{Status: models.PlayerNotFound}
		case err != nil && !have:
			return models.PlayerProfile{Status: models.PlayerUnknown}
		case err == nil:
			entry = mojangEntry{Key: key, UUID: id, Name: canonical, CheckedAt: m.now()}
			m.writeEntry(entry)
			fresh = true
		}
		// Out of reach with an older lookup kept: that is still the best answer.
	}
	// The face is asked for again with the lookup, and when it is missing.
	if when, ok := m.faceTime(entry.UUID); !ok || (fresh && m.now().Sub(when) >= mojangTTL) {
		if face, err := m.fetchFace(ctx, entry.UUID); err != nil {
			slog.Info("mojang face", "ok", false)
		} else if err := m.writeFace(entry.UUID, face); err != nil {
			slog.Warn("mojang: write face", "error", err)
		}
	}
	p := models.PlayerProfile{Name: entry.Name, UUID: dashUUID(entry.UUID), Status: models.PlayerFound}
	if when, ok := m.faceTime(entry.UUID); ok {
		// The version in the address lets the page see a face that changed.
		p.FaceSrc = fmt.Sprintf("%s%s.png?v=%d", mojangFaceRoute, entry.UUID, when.Unix())
	}
	return p
}

// errNoProfile is Mojang saying no profile has the name (404 or 204).
var errNoProfile = errors.New("mojang: no such profile")

// get makes one bounded GET and returns the body of a 200. A 204 or 404 is
// errNoProfile when none says so; anything else is errMojangUnreachable. The
// error never carries the request's address, which holds the name.
func (m *MojangService) get(ctx context.Context, target string, limit int64, noneIs404 bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, errMojangUnreachable
	}
	req.Header.Set("User-Agent", prismUserAgent)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, errMojangUnreachable
	}
	defer resp.Body.Close() //nolint:errcheck // read-only response
	switch {
	case resp.StatusCode == http.StatusOK:
	case noneIs404 && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent):
		return nil, errNoProfile
	default:
		return nil, errMojangUnreachable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, errMojangUnreachable
	}
	return body, nil
}

// lookup is the first request: the profile name to a UUID and Mojang's spelling.
func (m *MojangService) lookup(ctx context.Context, name string) (id, canonical string, err error) {
	ctx, cancel := context.WithTimeout(ctx, mojangTimeout)
	defer cancel()
	body, err := m.get(ctx, m.apiBase+"/users/profiles/minecraft/"+url.PathEscape(name), maxMojangLookup, true)
	if err != nil {
		return "", "", err
	}
	var doc struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", "", errMojangUnreachable
	}
	id = strings.ToLower(doc.ID)
	// Mojang's spelling must itself be a name, and the same one that was asked.
	if !mojangID.MatchString(id) || !ValidMinecraftName(doc.Name) || !strings.EqualFold(doc.Name, name) {
		return "", "", errMojangUnreachable
	}
	return id, doc.Name, nil
}

// skinHash holds a texture URL from the session server to the shape
// textures.minecraft.net serves: the exact host, no credentials, port or query,
// a hash path. Mojang writes these as http, so either scheme is read; the
// request is always made over https, to the host only (fetchFace).
func skinHash(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("not a URL")
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() != mojangTexturesHost ||
		u.Port() != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("not a textures.minecraft.net address")
	}
	hit := skinPath.FindStringSubmatch(u.EscapedPath())
	if hit == nil {
		return "", errors.New("not a texture path")
	}
	return hit[1], nil
}

// skinHashOf reads the skin's hash out of a session profile's textures property.
func skinHashOf(session []byte, id string) (string, error) {
	var doc struct {
		ID         string `json:"id"`
		Properties []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(session, &doc); err != nil || strings.ToLower(doc.ID) != id {
		return "", errors.New("not the profile asked for")
	}
	for _, p := range doc.Properties {
		if p.Name != "textures" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(p.Value)
		if err != nil || len(raw) > maxMojangSession {
			return "", errors.New("unreadable textures")
		}
		var tex struct {
			Textures struct {
				Skin *struct {
					URL string `json:"url"`
				} `json:"SKIN"`
			} `json:"textures"`
		}
		if err := json.Unmarshal(raw, &tex); err != nil || tex.Textures.Skin == nil {
			return "", errors.New("no skin")
		}
		return skinHash(tex.Textures.Skin.URL)
	}
	return "", errors.New("no textures")
}

// fetchFace is the second and third requests: the session profile for the skin's
// address, then the skin, cut down to the face.
func (m *MojangService) fetchFace(ctx context.Context, id string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*mojangTimeout)
	defer cancel()
	session, err := m.get(ctx, m.sessionBase+"/session/minecraft/profile/"+id, maxMojangSession, false)
	if err != nil {
		return nil, err
	}
	hash, err := skinHashOf(session, id)
	if err != nil {
		return nil, err
	}
	skin, err := m.get(ctx, m.textures+"/texture/"+hash, maxMojangSkin, false)
	if err != nil {
		return nil, err
	}
	return faceFromSkin(skin)
}

// faceFromSkin cuts the face out of a skin PNG: the 8x8 at (8,8), the hat
// layer at (40,8) laid over it where it has any alpha, scaled up nearest
// neighbour. The bytes must be a PNG of 64x64 or 64x32 (checked before the
// pixels are decoded), whatever the server's headers said.
func faceFromSkin(skin []byte) ([]byte, error) {
	if http.DetectContentType(skin) != "image/png" {
		return nil, errors.New("not a PNG")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(skin))
	if err != nil {
		return nil, err
	}
	if cfg.Width != 64 || (cfg.Height != 64 && cfg.Height != 32) {
		return nil, fmt.Errorf("a %dx%d image is not a skin", cfg.Width, cfg.Height)
	}
	img, err := png.Decode(bytes.NewReader(skin))
	if err != nil {
		return nil, err
	}
	src := image.NewNRGBA(img.Bounds())
	draw.Draw(src, src.Bounds(), img, img.Bounds().Min, draw.Src)

	face := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	draw.Draw(face, face.Bounds(), src, image.Pt(8, 8), draw.Src)
	hat := image.Rect(40, 8, 48, 16)
	// An old 64x32 skin has no alpha in its hat area unless it means it: the
	// game drops a hat region that is opaque throughout, and so does this.
	if cfg.Height == 32 && opaqueThroughout(src, hat) {
		hat = image.Rectangle{}
	}
	if !hat.Empty() {
		overlay := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		draw.Draw(overlay, overlay.Bounds(), src, hat.Min, draw.Src)
		draw.Draw(face, face.Bounds(), overlay, image.Point{}, draw.Over)
	}

	out := image.NewNRGBA(image.Rect(0, 0, 8*faceScale, 8*faceScale))
	for y := range out.Rect.Dy() {
		for x := range out.Rect.Dx() {
			out.SetNRGBA(x, y, face.NRGBAAt(x/faceScale, y/faceScale))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func opaqueThroughout(img *image.NRGBA, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if img.NRGBAAt(x, y).A < 128 {
				return false
			}
		}
	}
	return true
}

func dashUUID(id string) string {
	if len(id) != 32 {
		return id
	}
	return id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
}

// The cache. One lookup and the face of its UUID: a lookup for another name
// replaces both (forget and writeEntry), so the folder never holds more.

func (m *MojangService) readEntry() (mojangEntry, bool) {
	raw, err := os.ReadFile(filepath.Join(m.dir, mojangLookup))
	if err != nil {
		return mojangEntry{}, false
	}
	var e mojangEntry
	if json.Unmarshal(raw, &e) != nil || e.Key == "" || !mojangID.MatchString(e.UUID) || !ValidMinecraftName(e.Name) {
		return mojangEntry{}, false
	}
	return e, true
}

func (m *MojangService) writeEntry(e mojangEntry) {
	raw, err := json.Marshal(e)
	if err == nil {
		err = os.MkdirAll(m.dir, 0o700)
	}
	if err == nil {
		err = writeFileAtomic(filepath.Join(m.dir, mojangLookup), raw, 0o600)
	}
	if err != nil {
		slog.Warn("mojang: write lookup", "error", err)
		return
	}
	m.prune(e.UUID)
}

// writeFace stores the face of id, replacing the one there.
func (m *MojangService) writeFace(id string, face []byte) error {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(m.dir, id+".png"), face, 0o600)
}

// faceTime is when the cached face of id was written, and whether there is one.
func (m *MojangService) faceTime(id string) (time.Time, bool) {
	info, err := os.Stat(filepath.Join(m.dir, id+".png"))
	if err != nil || !info.Mode().IsRegular() {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

// forget removes everything kept: the name is not a profile.
func (m *MojangService) forget() { m.prune("") }

// prune removes every file of the cache except the lookup and the face of keep.
func (m *MojangService) prune(keep string) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || n == mojangLookup && keep != "" || keep != "" && n == keep+".png" {
			continue
		}
		if err := os.Remove(filepath.Join(m.dir, n)); err != nil {
			slog.Warn("mojang: prune", "error", err)
		}
	}
}

// FaceMiddleware serves the cached face at /mojang-face/<uuid>.png to the
// launcher's own page, ahead of the embedded assets, and hands every other
// request on. Only a GET or HEAD of one name shape is answered, read through an
// os.Root on the cache folder and re-checked as a PNG.
func (m *MojangService) FaceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		file, ok := strings.CutPrefix(r.URL.Path, mojangFaceRoute)
		if !ok {
			next.ServeHTTP(rw, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !faceFile.MatchString(file) {
			http.NotFound(rw, r)
			return
		}
		body, err := m.readFace(file)
		if err != nil || http.DetectContentType(body) != "image/png" {
			http.NotFound(rw, r)
			return
		}
		rw.Header().Set("Content-Type", "image/png")
		rw.Header().Set("X-Content-Type-Options", "nosniff")
		rw.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(rw, r, file, time.Time{}, bytes.NewReader(body))
	})
}

func (m *MojangService) readFace(file string) ([]byte, error) {
	root, err := os.OpenRoot(m.dir)
	if err != nil {
		return nil, err
	}
	defer root.Close() //nolint:errcheck // read-only
	return root.ReadFile(file)
}
