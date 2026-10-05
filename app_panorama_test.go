package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// writePanoramaPack writes a resources pack folder with six small faces.
func writePanoramaPack(t *testing.T, game, name string) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	for n := range 6 {
		path := filepath.Join(game, "resourcepacks", name,
			"assets", "minecraft", "textures", "gui", "title", "background", fmt.Sprintf("panorama_%d.png", n))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Only a chapter that is installed and has a resources pack with the six faces
// is answered, and the faces are served by the app's asset middleware.
func TestGetPanoramasAnswersInstalledChaptersWithAPack(t *testing.T) {
	app := newTestApp(t)
	game := installedFrangfurd(t, app)

	none, err := app.GetPanoramas()
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("no pack: got %v, %v; want an empty list and no error", none, err)
	}

	writePanoramaPack(t, game, "FrangfurdResources")
	got, err := app.GetPanoramas()
	if err != nil || len(got) != 1 || got[0].ChapterID != "frangfurd" {
		t.Fatalf("got %+v, %v", got, err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := app.assetMiddleware(next)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, got[0].Faces[2], nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("a face is not served: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/index.html", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("other paths must reach the next handler: %d", rec.Code)
	}
}
