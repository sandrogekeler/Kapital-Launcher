package services

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"kapital/backend/models"
)

// A chapter's title-screen panorama (issue 195, ADR-2's eleventh amendment).
// Each pack ships a resources pack of its own, a folder or a .zip in the
// instance's resourcepacks folder, with the six faces of the game's menu
// background at the vanilla path. The launcher lists that folder by name only,
// takes the first (sorted) entry whose name contains "resource" and which holds
// all six faces, opens just those six files (a zip's central directory is read
// and nothing else of it), checks each is a PNG of a sane size, all square and
// of one size, and keeps copies in its own cache, which it serves to its page
// at /panorama/<chapter>/panorama_<n>.png: the page's CSP stays closed, as for
// the wiki's pictures. Nothing else in a resources pack is opened.

const (
	panoramaDir   = "panorama"
	panoramaRoute = "/panorama/"
	// panoramaFaces is the cube's six sides.
	panoramaFaces = 6
	// maxPanoramaFace bounds one face's file. Vanilla's are 1024 pixels square
	// and well under 2 MiB.
	maxPanoramaFace = 4 << 20
	// maxPanoramaSide bounds a face's width and height, so a small file that
	// decodes to a huge image is refused before it is decoded.
	maxPanoramaSide = 4096
	// maxPackEntries bounds how many names of the resourcepacks folder are looked at.
	maxPackEntries = 512
	// panoramaKeyFile holds the source's key beside a chapter's cached faces.
	panoramaKeyFile = "key"
)

// panoramaFacePath is where a resources pack keeps face n, the game's own path.
func panoramaFacePath(n int) string {
	return fmt.Sprintf("assets/minecraft/textures/gui/title/background/panorama_%d.png", n)
}

// panoramaName is the name of a cached face below a chapter's folder, and the
// one shape the route answers for after the chapter id.
var panoramaName = regexp.MustCompile(`^panorama_[0-5]\.png$`)

// ErrNoPanorama is returned for a pack that does not hold all six faces, or
// whose faces are not six PNGs of one square size.
var ErrNoPanorama = errors.New("not a panorama")

// PanoramaService finds, validates and caches the panoramas.
type PanoramaService struct {
	mu  sync.Mutex
	dir string
}

// NewPanoramaService caches under dataDir/panorama.
func NewPanoramaService(dataDir string) *PanoramaService {
	return &PanoramaService{dir: filepath.Join(dataDir, panoramaDir)}
}

// PanoramaSource is a chapter and its instance folder, "" when it is not installed.
type PanoramaSource struct {
	ChapterID   string
	InstanceDir string
}

// Refresh returns the panorama of each source that has one, in the order given,
// and removes the cache of every other chapter. It looks again whenever it is
// called (the page asks on window focus and after an install or a run, never on
// a timer); a chapter whose source has not changed since the last call is
// answered from the cache without reading a face.
func (p *PanoramaService) Refresh(sources []PanoramaSource) []models.Panorama {
	p.mu.Lock()
	defer p.mu.Unlock()
	found := []models.Panorama{}
	for _, s := range sources {
		if !chapterIDPattern.MatchString(s.ChapterID) {
			continue
		}
		if pano, ok := p.forChapter(s); ok {
			found = append(found, pano)
			continue
		}
		p.drop(s.ChapterID)
	}
	return found
}

// drop removes a chapter's cached faces, the launcher's own files.
func (p *PanoramaService) drop(id string) {
	target := filepath.Join(p.dir, id)
	if _, err := os.Stat(target); err != nil {
		return
	}
	if err := os.RemoveAll(target); err != nil {
		slog.Warn("panorama: remove cache", "chapter", id, "error", err)
	}
}

// forChapter walks the chapter's resource packs in name order and caches the
// first that is a panorama.
func (p *PanoramaService) forChapter(s PanoramaSource) (models.Panorama, bool) {
	game := gameFolder(s.InstanceDir)
	if game == "" {
		return models.Panorama{}, false
	}
	root, err := os.OpenRoot(filepath.Join(game, "resourcepacks"))
	if err != nil {
		return models.Panorama{}, false
	}
	defer root.Close() //nolint:errcheck // read-only
	for _, name := range listPanoramaPacks(root) {
		src, err := openPanoramaPack(root, name)
		if err != nil {
			continue
		}
		pano, err := p.cache(s.ChapterID, src)
		src.close()
		if err != nil {
			slog.Warn("panorama: pack skipped", "chapter", s.ChapterID, "error", err)
			continue
		}
		return pano, true
	}
	return models.Panorama{}, false
}

// listPanoramaPacks is the sorted names of the resourcepacks folder's folders
// and .zip files whose name contains "resource", in any case. Names only: no
// file is opened here.
func listPanoramaPacks(root *os.Root) []string {
	dir, err := root.Open(".")
	if err != nil {
		return nil
	}
	defer dir.Close() //nolint:errcheck // read-only
	entries, err := dir.ReadDir(maxPackEntries)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if !strings.Contains(strings.ToLower(name), "resource") {
			continue
		}
		isZip := e.Type().IsRegular() && strings.EqualFold(filepath.Ext(name), ".zip")
		if e.IsDir() || isZip {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// panoramaPack is an opened resources pack: its name, a stamp of the files'
// sizes and times that changes when the faces may have, and a reader for each
// of the six.
type panoramaPack struct {
	name  string
	stamp string
	read  func(face int) ([]byte, error)
	close func()
}

// openPanoramaPack opens a folder or a zip by its name in the root, and refuses
// one that does not hold all six faces as regular files.
func openPanoramaPack(root *os.Root, name string) (*panoramaPack, error) {
	info, err := root.Stat(name)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return openFolderPack(root, name)
	}
	if info.Mode().IsRegular() {
		return openZipPack(root, name, info)
	}
	return nil, ErrNoPanorama
}

func openFolderPack(root *os.Root, name string) (*panoramaPack, error) {
	sub, err := root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	var stamp strings.Builder
	for n := range panoramaFaces {
		info, err := sub.Stat(panoramaFacePath(n))
		if err != nil || !info.Mode().IsRegular() {
			sub.Close() //nolint:errcheck // read-only
			return nil, ErrNoPanorama
		}
		fmt.Fprintf(&stamp, "%d:%d:%d;", n, info.Size(), info.ModTime().UnixNano())
	}
	return &panoramaPack{
		name:  name,
		stamp: stamp.String(),
		read: func(face int) ([]byte, error) {
			f, err := sub.Open(panoramaFacePath(face))
			if err != nil {
				return nil, err
			}
			defer f.Close() //nolint:errcheck // read-only
			return readFace(f)
		},
		close: func() { sub.Close() }, //nolint:errcheck // read-only
	}, nil
}

func openZipPack(root *os.Root, name string, info os.FileInfo) (*panoramaPack, error) {
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	// The central directory is all that is read here.
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		f.Close() //nolint:errcheck // read-only
		return nil, err
	}
	var faces [panoramaFaces]*zip.File
	want := map[string]int{}
	for n := range panoramaFaces {
		want[panoramaFacePath(n)] = n
	}
	for _, entry := range zr.File {
		if n, ok := want[entry.Name]; ok && !entry.FileInfo().IsDir() {
			faces[n] = entry
		}
	}
	for _, entry := range faces {
		if entry == nil || entry.UncompressedSize64 > maxPanoramaFace {
			f.Close() //nolint:errcheck // read-only
			return nil, ErrNoPanorama
		}
	}
	return &panoramaPack{
		name:  name,
		stamp: fmt.Sprintf("zip:%d:%d;", info.Size(), info.ModTime().UnixNano()),
		read: func(face int) ([]byte, error) {
			rc, err := faces[face].Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close() //nolint:errcheck // read-only
			return readFace(rc)
		},
		close: func() { f.Close() }, //nolint:errcheck // read-only
	}, nil
}

// readFace reads one face up to the cap and refuses a longer one.
func readFace(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxPanoramaFace+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPanoramaFace {
		return nil, fmt.Errorf("%w: a face is over %d bytes", ErrNoPanorama, maxPanoramaFace)
	}
	return body, nil
}

// cache makes the chapter's cache hold this pack's faces, reading them only
// when the key (the pack's name and its stamp) is not the one cached, and
// returns where the page loads them.
func (p *PanoramaService) cache(id string, src *panoramaPack) (models.Panorama, error) {
	sum := sha256.Sum256([]byte(src.name + "|" + src.stamp))
	key := hex.EncodeToString(sum[:8])
	dir := filepath.Join(p.dir, id)
	if p.fresh(dir, key) {
		return panoramaFor(id, key), nil
	}
	bodies := make([][]byte, panoramaFaces)
	side := 0
	for n := range panoramaFaces {
		body, err := src.read(n)
		if err != nil {
			return models.Panorama{}, err
		}
		got, err := checkFace(body)
		if err != nil {
			return models.Panorama{}, fmt.Errorf("face %d: %w", n, err)
		}
		if n > 0 && got != side {
			return models.Panorama{}, fmt.Errorf("%w: faces are not one size", ErrNoPanorama)
		}
		side, bodies[n] = got, body
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return models.Panorama{}, err
	}
	// The key goes last, so a half-written cache is never taken for a whole one.
	if err := os.Remove(filepath.Join(dir, panoramaKeyFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return models.Panorama{}, err
	}
	for n, body := range bodies {
		if err := writeFileAtomic(filepath.Join(dir, fmt.Sprintf("panorama_%d.png", n)), body, 0o600); err != nil {
			return models.Panorama{}, err
		}
	}
	if err := writeFileAtomic(filepath.Join(dir, panoramaKeyFile), []byte(key), 0o600); err != nil {
		return models.Panorama{}, err
	}
	slog.Info("panorama cached", "chapter", id, "side", side)
	return panoramaFor(id, key), nil
}

// fresh says whether the chapter's cache was made from this key and still has
// its six files.
func (p *PanoramaService) fresh(dir, key string) bool {
	got, err := os.ReadFile(filepath.Join(dir, panoramaKeyFile))
	if err != nil || string(got) != key {
		return false
	}
	for n := range panoramaFaces {
		info, err := os.Stat(filepath.Join(dir, fmt.Sprintf("panorama_%d.png", n)))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// panoramaFor is the answer for a chapter: the six paths, each carrying the key
// so a changed source is a new address and the webview's own copy is not reused.
func panoramaFor(id, key string) models.Panorama {
	pano := models.Panorama{ChapterID: id}
	for n := range panoramaFaces {
		pano.Faces[n] = fmt.Sprintf("%s%s/panorama_%d.png?v=%s", panoramaRoute, id, n, key)
	}
	return pano
}

// checkFace decodes a face as a PNG and returns its side. A file that is not a
// PNG, is not square or is past the size cap is refused; the dimensions are
// read before the pixels, so a small file cannot ask for a huge image.
func checkFace(body []byte) (int, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("%w: not a PNG", ErrNoPanorama)
	}
	if cfg.Width != cfg.Height || cfg.Width < 1 || cfg.Width > maxPanoramaSide {
		return 0, fmt.Errorf("%w: a face is %dx%d", ErrNoPanorama, cfg.Width, cfg.Height)
	}
	if _, err := png.Decode(bytes.NewReader(body)); err != nil {
		return 0, fmt.Errorf("%w: damaged PNG", ErrNoPanorama)
	}
	return cfg.Width, nil
}

// panoramaRouteName splits what follows /panorama/ into the chapter id and the
// face's file name, and says whether it has the one shape served.
func panoramaRouteName(rest string) (id, file string, ok bool) {
	if path.Clean(rest) != rest {
		return "", "", false
	}
	id, file, found := strings.Cut(rest, "/")
	if !found || !chapterIDPattern.MatchString(id) || !panoramaName.MatchString(file) {
		return "", "", false
	}
	return id, file, true
}

// Middleware serves the cached faces at /panorama/<chapter>/panorama_<n>.png to
// the launcher's own page, ahead of the embedded assets, and hands every other
// request on. Only a GET or HEAD of that name shape is answered, read through
// an os.Root on the cache folder, and only a body that is still a PNG.
func (p *PanoramaService) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rest, ok := strings.CutPrefix(r.URL.Path, panoramaRoute)
		if !ok {
			next.ServeHTTP(rw, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id, file, ok := panoramaRouteName(rest)
		if !ok {
			http.NotFound(rw, r)
			return
		}
		body, err := p.read(id + "/" + file)
		if err != nil || !bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")) {
			http.NotFound(rw, r)
			return
		}
		rw.Header().Set("Content-Type", "image/png")
		rw.Header().Set("X-Content-Type-Options", "nosniff")
		rw.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(rw, r, file, time.Time{}, bytes.NewReader(body))
	})
}

// read reads one cached file by its place below the cache folder, through an
// os.Root on that folder.
func (p *PanoramaService) read(rel string) ([]byte, error) {
	root, err := os.OpenRoot(p.dir)
	if err != nil {
		return nil, err
	}
	defer root.Close() //nolint:errcheck // read-only
	return root.ReadFile(rel)
}
