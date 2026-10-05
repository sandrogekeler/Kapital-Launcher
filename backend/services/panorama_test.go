package services

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// panoPNG is a PNG of a given size whose colour says which face it is.
func panoPNG(t *testing.T, w, h int, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: shade, G: 80, B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// panoRig is an instance folder with a game folder, and a service caching in
// its own temp dir.
type panoRig struct {
	t        *testing.T
	instance string
	game     string
	svc      *PanoramaService
	data     string
}

func newPanoRig(t *testing.T) *panoRig {
	t.Helper()
	instance := t.TempDir()
	game := filepath.Join(instance, "minecraft")
	if err := os.MkdirAll(filepath.Join(game, "resourcepacks"), 0o755); err != nil {
		t.Fatal(err)
	}
	data := t.TempDir()
	return &panoRig{t: t, instance: instance, game: game, svc: NewPanoramaService(data), data: data}
}

// faces are six PNGs of one size, shade 10 apart.
func (r *panoRig) faces(side int) [][]byte {
	out := make([][]byte, panoramaFaces)
	for n := range out {
		out[n] = panoPNG(r.t, side, side, uint8(10*n))
	}
	return out
}

// folder writes a resources pack folder holding the given faces (nil skips one).
func (r *panoRig) folder(name string, faces [][]byte) string {
	r.t.Helper()
	for n, body := range faces {
		if body == nil {
			continue
		}
		path := filepath.Join(r.game, "resourcepacks", name, filepath.FromSlash(panoramaFacePath(n)))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			r.t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			r.t.Fatal(err)
		}
	}
	return filepath.Join(r.game, "resourcepacks", name)
}

// zipPack writes a resources pack zip, with an unrelated entry beside the faces.
func (r *panoRig) zipPack(name string, faces [][]byte) {
	r.t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	put := func(entry string, body []byte) {
		w, err := zw.Create(entry)
		if err != nil {
			r.t.Fatal(err)
		}
		if _, err := w.Write(body); err != nil {
			r.t.Fatal(err)
		}
	}
	put("pack.mcmeta", []byte(`{"pack":{"pack_format":15}}`))
	for n, body := range faces {
		if body != nil {
			put(panoramaFacePath(n), body)
		}
	}
	if err := zw.Close(); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.game, "resourcepacks", name), buf.Bytes(), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *panoRig) refresh() (string, bool) {
	r.t.Helper()
	got := r.svc.Refresh([]PanoramaSource{{ChapterID: "frangfurd", InstanceDir: r.instance}})
	if len(got) == 0 {
		return "", false
	}
	if got[0].ChapterID != "frangfurd" {
		r.t.Fatalf("chapter %q", got[0].ChapterID)
	}
	return got[0].Faces[0], true
}

func (r *panoRig) cached(n int) []byte {
	r.t.Helper()
	body, err := os.ReadFile(filepath.Join(r.data, panoramaDir, "frangfurd", "panorama_"+string(rune('0'+n))+".png"))
	if err != nil {
		r.t.Fatal(err)
	}
	return body
}

func TestPanoramaFromAFolderPackIsCachedAndAddressed(t *testing.T) {
	r := newPanoRig(t)
	faces := r.faces(8)
	r.folder("FrangfurdResources", faces)
	got := r.svc.Refresh([]PanoramaSource{{ChapterID: "frangfurd", InstanceDir: r.instance}})
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	for n, src := range got[0].Faces {
		want := "/panorama/frangfurd/panorama_" + string(rune('0'+n)) + ".png?v="
		if !strings.HasPrefix(src, want) || len(src) <= len(want) {
			t.Errorf("face %d: %q", n, src)
		}
		if !bytes.Equal(r.cached(n), faces[n]) {
			t.Errorf("face %d: the cache is not a copy of the pack's file", n)
		}
	}
}

func TestPanoramaFromAZipPackReadsTheSixEntries(t *testing.T) {
	r := newPanoRig(t)
	faces := r.faces(8)
	r.zipPack("Lichdenstein-Resources.ZIP", faces)
	if _, ok := r.refresh(); !ok {
		t.Fatal("no panorama from the zip")
	}
	for n := range faces {
		if !bytes.Equal(r.cached(n), faces[n]) {
			t.Errorf("face %d differs", n)
		}
	}
}

func TestPanoramaUsesTheOlderGameFolderName(t *testing.T) {
	r := newPanoRig(t)
	if err := os.Rename(r.game, filepath.Join(r.instance, ".minecraft")); err != nil {
		t.Fatal(err)
	}
	r.game = filepath.Join(r.instance, ".minecraft")
	r.folder("Some Resources", r.faces(8))
	if _, ok := r.refresh(); !ok {
		t.Fatal("no panorama from .minecraft")
	}
}

func TestPanoramaPicksTheFirstSortedPackWithAResourceName(t *testing.T) {
	r := newPanoRig(t)
	other := r.faces(8)
	for n := range other {
		other[n] = panoPNG(t, 8, 8, uint8(200+n))
	}
	// Not named "resource": ignored even though it holds the faces.
	r.folder("AAA Shaders", other)
	// Named for it, but missing a face: skipped.
	partial := r.faces(8)
	partial[3] = nil
	r.folder("Bravo resources", partial)
	// The first sorted that holds all six.
	want := r.faces(8)
	r.folder("Charlie RESOURCES", want)
	r.zipPack("Delta resources.zip", other)
	if _, ok := r.refresh(); !ok {
		t.Fatal("no panorama")
	}
	if !bytes.Equal(r.cached(0), want[0]) {
		t.Fatal("not the first sorted pack that holds six faces")
	}
}

func TestPanoramaRefusesWhatIsNotSixSquarePNGsOfOneSize(t *testing.T) {
	big := make([]byte, maxPanoramaFace+1)
	copy(big, panoPNG(t, 8, 8, 1))
	tests := []struct {
		name   string
		change func(f [][]byte)
	}{
		{"not a png", func(f [][]byte) { f[2] = []byte("GIF89a not a png") }},
		{"truncated png", func(f [][]byte) { f[1] = f[1][:len(f[1])/2] }},
		{"not square", func(f [][]byte) { f[4] = panoPNG(t, 8, 4, 1) }},
		{"another size", func(f [][]byte) { f[5] = panoPNG(t, 16, 16, 1) }},
		{"too many pixels", func(f [][]byte) { f[0] = panoPNG(t, maxPanoramaSide+1, maxPanoramaSide+1, 1) }},
		{"over the size cap", func(f [][]byte) { f[3] = big }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newPanoRig(t)
			faces := r.faces(8)
			tt.change(faces)
			r.folder("Pack Resources", faces)
			if _, ok := r.refresh(); ok {
				t.Fatal("a panorama was offered")
			}
			if _, err := os.Stat(filepath.Join(r.data, panoramaDir, "frangfurd", "panorama_0.png")); err == nil {
				t.Fatal("a refused pack left a cached face")
			}
		})
	}
}

func TestPanoramaZipRefusesAnOversizeEntry(t *testing.T) {
	r := newPanoRig(t)
	faces := r.faces(8)
	faces[2] = append(panoPNG(t, 8, 8, 1), make([]byte, maxPanoramaFace)...)
	r.zipPack("Big Resources.zip", faces)
	if _, ok := r.refresh(); ok {
		t.Fatal("an oversize entry was read")
	}
}

func TestPanoramaNeedsAnInstalledChapterWithAPack(t *testing.T) {
	r := newPanoRig(t)
	// No resourcepacks folder at all.
	if err := os.RemoveAll(filepath.Join(r.game, "resourcepacks")); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.refresh(); ok {
		t.Fatal("a panorama without a folder")
	}
	// A chapter that is not installed.
	got := r.svc.Refresh([]PanoramaSource{{ChapterID: "lichdenstein"}, {ChapterID: "../x", InstanceDir: r.instance}})
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestPanoramaIsReadAgainOnlyWhenTheSourceChanged(t *testing.T) {
	r := newPanoRig(t)
	r.folder("FrangfurdResources", r.faces(8))
	first, ok := r.refresh()
	if !ok {
		t.Fatal("no panorama")
	}
	// Unchanged: answered from the cache, which a tampered copy shows.
	cachedFace := filepath.Join(r.data, panoramaDir, "frangfurd", "panorama_0.png")
	if err := os.WriteFile(cachedFace, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	again, _ := r.refresh()
	if again != first || string(r.cached(0)) != "tampered" {
		t.Fatalf("an unchanged source was read again: %q %q", again, first)
	}
	// A face rewritten with a new time: a new key, and the copy is replaced.
	newFaces := r.faces(8)
	newFaces[0] = panoPNG(t, 8, 8, 99)
	pack := r.folder("FrangfurdResources", newFaces)
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(pack, filepath.FromSlash(panoramaFacePath(0))), later, later); err != nil {
		t.Fatal(err)
	}
	changed, ok := r.refresh()
	if !ok || changed == first {
		t.Fatalf("a changed source kept its key: %q", changed)
	}
	if !bytes.Equal(r.cached(0), newFaces[0]) {
		t.Fatal("the cache was not refreshed")
	}
}

func TestPanoramaCacheIsRemovedWhenThePackGoes(t *testing.T) {
	r := newPanoRig(t)
	pack := r.folder("FrangfurdResources", r.faces(8))
	if _, ok := r.refresh(); !ok {
		t.Fatal("no panorama")
	}
	if err := os.RemoveAll(pack); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.refresh(); ok {
		t.Fatal("a panorama after the pack went")
	}
	if _, err := os.Stat(filepath.Join(r.data, panoramaDir, "frangfurd")); err == nil {
		t.Fatal("the chapter's cache stayed")
	}
}

func TestPanoramaPackThatIsASymlinkOutOfTheFolderIsNotFollowed(t *testing.T) {
	r := newPanoRig(t)
	outside := t.TempDir()
	for n, body := range r.faces(8) {
		path := filepath.Join(outside, filepath.FromSlash(panoramaFacePath(n)))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(r.game, "resourcepacks", "Linked Resources")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if _, ok := r.refresh(); ok {
		t.Fatal("a link out of the resourcepacks folder was followed")
	}
}

func TestPanoramaRouteName(t *testing.T) {
	tests := []struct {
		rest string
		ok   bool
	}{
		{"frangfurd/panorama_0.png", true},
		{"lichdenstein/panorama_5.png", true},
		{"frangfurd/panorama_6.png", false},
		{"frangfurd/panorama_0.png/", false},
		{"frangfurd/key", false},
		{"frangfurd/../frangfurd/panorama_0.png", false},
		{"../panorama_0.png", false},
		{"Frangfurd/panorama_0.png", false},
		{"frangfurd//panorama_0.png", false},
		{"frangfurd", false},
		{"", false},
	}
	for _, tt := range tests {
		if _, _, ok := panoramaRouteName(tt.rest); ok != tt.ok {
			t.Errorf("%q: got %v, want %v", tt.rest, ok, tt.ok)
		}
	}
}

func TestPanoramaMiddlewareServesTheCachedFacesOnly(t *testing.T) {
	r := newPanoRig(t)
	faces := r.faces(8)
	r.folder("FrangfurdResources", faces)
	if _, ok := r.refresh(); !ok {
		t.Fatal("no panorama")
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("next")) }) //nolint:errcheck // a test handler
	h := r.svc.Middleware(next)
	do := func(method, target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
		return rec
	}

	get := do(http.MethodGet, "/panorama/frangfurd/panorama_3.png?v=abc")
	if get.Code != http.StatusOK || get.Header().Get("Content-Type") != "image/png" || !bytes.Equal(get.Body.Bytes(), faces[3]) {
		t.Fatalf("GET: %d %q", get.Code, get.Header().Get("Content-Type"))
	}
	if get.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("no nosniff")
	}
	if head := do(http.MethodHead, "/panorama/frangfurd/panorama_3.png"); head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD: %d, %d bytes", head.Code, head.Body.Len())
	}
	if post := do(http.MethodPost, "/panorama/frangfurd/panorama_3.png"); post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", post.Code)
	}
	for _, target := range []string{
		"/panorama/frangfurd/key",
		"/panorama/frangfurd/panorama_9.png",
		"/panorama/lichdenstein/panorama_0.png",
		"/panorama/frangfurd/../frangfurd/panorama_0.png",
		"/panorama/",
		"/panorama/frangfurd",
	} {
		if rec := do(http.MethodGet, target); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", target, rec.Code)
		}
	}
	// Anything else is the next handler's.
	if rec := do(http.MethodGet, "/index.html"); rec.Body.String() != "next" {
		t.Fatalf("other path: %q", rec.Body.String())
	}
	// A cached file that is no longer a PNG is not served.
	if err := os.WriteFile(filepath.Join(r.data, panoramaDir, "frangfurd", "panorama_1.png"), []byte("<html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if rec := do(http.MethodGet, "/panorama/frangfurd/panorama_1.png"); rec.Code != http.StatusNotFound {
		t.Fatalf("a non-PNG was served: %d", rec.Code)
	}
}
