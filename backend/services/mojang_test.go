package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kapital/backend/models"
)

const (
	testUUID    = "069a79f444e94726a5befca90e38aaf5"
	testDashed  = "069a79f4-44e9-4726-a5be-fca90e38aaf5"
	testSkinHex = "292009a4925b58f02c77dadc3ecef07ea4c7472f64e0fdc32ce5522489362680"
)

var (
	faceRed = color.NRGBA{R: 200, A: 255}
	hatBlue = color.NRGBA{B: 220, A: 255}
)

// skinPNG draws a skin of the given height whose face is red. The hat layer is
// transparent but for one blue pixel at its top left, or, with opaqueHat, solid
// green throughout.
func skinPNG(t *testing.T, height int, opaqueHat bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 64, height))
	for y := 8; y < 16; y++ {
		for x := 8; x < 16; x++ {
			img.SetNRGBA(x, y, faceRed)
		}
	}
	if opaqueHat {
		for y := 8; y < 16; y++ {
			for x := 40; x < 48; x++ {
				img.SetNRGBA(x, y, color.NRGBA{G: 255, A: 255})
			}
		}
	} else {
		img.SetNRGBA(40, 8, hatBlue)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// mojangFake answers the three endpoints and counts the requests to each.
type mojangFake struct {
	srv                      *httptest.Server
	lookups, sessions, skins atomic.Int32
	lookupStatus, skinStatus atomic.Int32
	skin                     atomic.Value // []byte
	agent, lastLookupPath    atomic.Value // string
	textureURL               atomic.Value // string, "" for the real shape
}

func newMojangFake(t *testing.T) *mojangFake {
	t.Helper()
	f := &mojangFake{}
	f.lookupStatus.Store(http.StatusOK)
	f.skinStatus.Store(http.StatusOK)
	f.skin.Store(skinPNG(t, 64, false))
	f.textureURL.Store("")
	mux := http.NewServeMux()
	mux.HandleFunc("/users/profiles/minecraft/", func(w http.ResponseWriter, r *http.Request) {
		f.lookups.Add(1)
		f.agent.Store(r.Header.Get("User-Agent"))
		f.lastLookupPath.Store(r.URL.Path)
		if st := int(f.lookupStatus.Load()); st != http.StatusOK {
			w.WriteHeader(st)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/users/profiles/minecraft/")
		// The id comes back with an upper-case start, which must be read as lower case.
		fmt.Fprintf(w, `{"id":%q,"name":%q}`, strings.ToUpper(testUUID[:4])+testUUID[4:], canonicalOf(name))
	})
	mux.HandleFunc("/session/minecraft/profile/", func(w http.ResponseWriter, r *http.Request) {
		f.sessions.Add(1)
		u, _ := f.textureURL.Load().(string)
		if u == "" {
			// Mojang writes these as http.
			u = "http://textures.minecraft.net/texture/" + testSkinHex
		}
		tex := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"textures":{"SKIN":{"url":%q}}}`, u)))
		fmt.Fprintf(w, `{"id":%q,"name":"x","properties":[{"name":"textures","value":%q}]}`, testUUID, tex)
	})
	mux.HandleFunc("/texture/", func(w http.ResponseWriter, r *http.Request) {
		f.skins.Add(1)
		if st := int(f.skinStatus.Load()); st != http.StatusOK {
			w.WriteHeader(st)
			return
		}
		if r.URL.Path != "/texture/"+testSkinHex {
			http.NotFound(w, r)
			return
		}
		body, _ := f.skin.Load().([]byte)
		if _, err := w.Write(body); err != nil {
			t.Error(err)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// canonicalOf is Mojang's spelling: "snadrochka" comes back capitalised.
func canonicalOf(name string) string {
	if strings.EqualFold(name, "snadrochka") {
		return "Snadrochka"
	}
	return name
}

func (f *mojangFake) service(t *testing.T) *MojangService {
	t.Helper()
	m := NewMojangService(t.TempDir())
	m.apiBase, m.sessionBase, m.textures = f.srv.URL, f.srv.URL, f.srv.URL
	return m
}

func TestMojangProfileFindsTheUUIDAndCutsTheFace(t *testing.T) {
	f := newMojangFake(t)
	m := f.service(t)
	p := m.Profile(context.Background(), "snadrochka")
	if p.Status != models.PlayerFound || p.Name != "Snadrochka" || p.UUID != testDashed {
		t.Fatalf("got %+v", p)
	}
	if !strings.HasPrefix(p.FaceSrc, "/mojang-face/"+testUUID+".png?v=") {
		t.Fatalf("face src %q", p.FaceSrc)
	}
	if got, _ := f.agent.Load().(string); got != prismUserAgent {
		t.Fatalf("user agent %q", got)
	}
	raw, err := os.ReadFile(filepath.Join(m.dir, testUUID+".png"))
	if err != nil {
		t.Fatal(err)
	}
	face, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if b := face.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("face is %v", b)
	}
	// The hat pixel covers the first 8x8 block, the rest is the face.
	if got := color.NRGBAModel.Convert(face.At(3, 3)); got != hatBlue {
		t.Fatalf("hat block is %v", got)
	}
	if got := color.NRGBAModel.Convert(face.At(40, 40)); got != faceRed {
		t.Fatalf("face block is %v", got)
	}
}

func TestMojangProfileRefusesAnInvalidNameWithoutAsking(t *testing.T) {
	f := newMojangFake(t)
	m := f.service(t)
	for _, name := range []string{"", "ab", "a b c", "has/slash", "../x", "seventeen_chars_xx", "Ünicode", "x?y=z"} {
		if p := m.Profile(context.Background(), name); p.Status != models.PlayerInvalid {
			t.Errorf("%q: %+v", name, p)
		}
	}
	if f.lookups.Load() != 0 {
		t.Fatal("an invalid name reached the network")
	}
}

func TestMojangProfileNotFoundOn404And204AndForgets(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusNoContent} {
		f := newMojangFake(t)
		m := f.service(t)
		if p := m.Profile(context.Background(), "Snadrochka"); p.Status != models.PlayerFound {
			t.Fatalf("setup: %+v", p)
		}
		m.now = func() time.Time { return time.Now().Add(48 * time.Hour) }
		f.lookupStatus.Store(int32(status))
		p := m.Profile(context.Background(), "Snadrochka")
		if p != (models.PlayerProfile{Status: models.PlayerNotFound}) {
			t.Fatalf("%d: %+v", status, p)
		}
		if entries, _ := os.ReadDir(m.dir); len(entries) != 0 {
			t.Fatalf("%d: the cache kept %d files", status, len(entries))
		}
		if _, err := m.CopyUUID(); err == nil {
			t.Fatal("a name with no profile has a UUID to copy")
		}
	}
}

func TestMojangProfileUnknownWhenOffline(t *testing.T) {
	f := newMojangFake(t)
	m := f.service(t)
	f.srv.Close()
	if p := m.Profile(context.Background(), "Snadrochka"); p != (models.PlayerProfile{Status: models.PlayerUnknown}) {
		t.Fatalf("got %+v", p)
	}
	// A rate limit, a server error and a refusal are the same: no answer, not an error.
	f2 := newMojangFake(t)
	m2 := f2.service(t)
	for _, st := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadRequest} {
		f2.lookupStatus.Store(int32(st))
		if p := m2.Profile(context.Background(), "Snadrochka"); p.Status != models.PlayerUnknown {
			t.Fatalf("%d: %+v", st, p)
		}
	}
}

func TestMojangProfileUsesTheCacheForADayThenKeepsItOffline(t *testing.T) {
	f := newMojangFake(t)
	m := f.service(t)
	now := time.Now()
	m.now = func() time.Time { return now }
	first := m.Profile(context.Background(), "Snadrochka")
	m.Profile(context.Background(), "snadrochka")
	if f.lookups.Load() != 1 || f.skins.Load() != 1 {
		t.Fatalf("a fresh lookup asked again: %d lookups, %d skins", f.lookups.Load(), f.skins.Load())
	}
	// A day later it is asked again, and the skin is cut again.
	now = now.Add(mojangTTL + time.Hour)
	m.Profile(context.Background(), "Snadrochka")
	if f.lookups.Load() != 2 || f.skins.Load() != 2 {
		t.Fatalf("a stale lookup was not asked again: %d lookups, %d skins", f.lookups.Load(), f.skins.Load())
	}
	// Offline after another day: the kept lookup and face still answer.
	now = now.Add(mojangTTL + time.Hour)
	f.srv.Close()
	p := m.Profile(context.Background(), "Snadrochka")
	if p.Status != models.PlayerFound || p.UUID != first.UUID || p.FaceSrc == "" {
		t.Fatalf("offline with a cache: %+v", p)
	}
}

func TestMojangProfileForADifferentNameReplacesTheCache(t *testing.T) {
	f := newMojangFake(t)
	m := f.service(t)
	m.Profile(context.Background(), "Snadrochka")
	// Another profile's face, left in the folder, must not outlive the next lookup.
	other := filepath.Join(m.dir, strings.Repeat("a", 32)+".png")
	if err := os.WriteFile(other, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.Profile(context.Background(), "Another_Name")
	if _, err := os.Stat(other); !os.IsNotExist(err) {
		t.Fatalf("an old face stayed: %v", err)
	}
	if e, ok := m.readEntry(); !ok || e.Key != "another_name" {
		t.Fatalf("entry %+v", e)
	}
}

func TestMojangProfileWithoutAFaceStaysFoundWithNoFaceSrc(t *testing.T) {
	cases := map[string]func(f *mojangFake){
		"skin 404":       func(f *mojangFake) { f.skinStatus.Store(http.StatusNotFound) },
		"not a PNG":      func(f *mojangFake) { f.skin.Store([]byte("GIF89a not a png")) },
		"too large":      func(f *mojangFake) { f.skin.Store(append(skinPNG(t, 64, false), make([]byte, maxMojangSkin)...)) },
		"wrong size":     func(f *mojangFake) { f.skin.Store(wrongSizePNG(t)) },
		"foreign host":   func(f *mojangFake) { f.textureURL.Store("https://evil.example/texture/" + testSkinHex) },
		"traversal path": func(f *mojangFake) { f.textureURL.Store("http://textures.minecraft.net/texture/../../x") },
	}
	for name, setup := range cases {
		f := newMojangFake(t)
		setup(f)
		m := f.service(t)
		p := m.Profile(context.Background(), "Snadrochka")
		if p.Status != models.PlayerFound || p.UUID != testDashed || p.FaceSrc != "" {
			t.Errorf("%s: %+v", name, p)
		}
		if _, err := os.Stat(filepath.Join(m.dir, testUUID+".png")); err == nil {
			t.Errorf("%s: a face was kept", name)
		}
	}
}

func wrongSizePNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 4000, 4000))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestMojangLookupRefusesAnAnswerForAnotherNameOrABadID(t *testing.T) {
	for name, body := range map[string]string{
		"another name": `{"id":"` + testUUID + `","name":"Someone_Else"}`,
		"short id":     `{"id":"abc","name":"Snadrochka"}`,
		"not json":     `<html>`,
		"traversal id": `{"id":"../../etc/passwd/../../0000000","name":"Snadrochka"}`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, body) }))
		m := NewMojangService(t.TempDir())
		m.apiBase = srv.URL
		if p := m.Profile(context.Background(), "Snadrochka"); p.Status != models.PlayerUnknown {
			t.Errorf("%s: %+v", name, p)
		}
		srv.Close()
	}
}

func TestMojangFollowsNoRedirect(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		fmt.Fprintf(w, `{"id":%q,"name":"Snadrochka"}`, testUUID)
	}))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer srv.Close()
	m := NewMojangService(t.TempDir())
	m.apiBase = srv.URL
	if p := m.Profile(context.Background(), "Snadrochka"); p.Status != models.PlayerUnknown || hits.Load() != 0 {
		t.Fatalf("%+v, redirect target hit %d times", p, hits.Load())
	}
}

func TestMojangBodiesAreBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"id":%q,"name":"Snadrochka","pad":%q}`, testUUID, strings.Repeat("x", maxMojangLookup))
	}))
	defer srv.Close()
	m := NewMojangService(t.TempDir())
	m.apiBase = srv.URL
	if p := m.Profile(context.Background(), "Snadrochka"); p.Status != models.PlayerUnknown {
		t.Fatalf("an oversized answer was read: %+v", p)
	}
}

func TestMojangLookupPathCarriesTheName(t *testing.T) {
	f := newMojangFake(t)
	m := f.service(t)
	m.Profile(context.Background(), "Snadrochka")
	if got, _ := f.lastLookupPath.Load().(string); got != "/users/profiles/minecraft/Snadrochka" {
		t.Fatalf("lookup path %q", got)
	}
}

func TestSkinHashAcceptsOnlyTexturesMinecraftNet(t *testing.T) {
	good := []string{
		"http://textures.minecraft.net/texture/" + testSkinHex,
		"https://textures.minecraft.net/texture/" + testSkinHex,
	}
	for _, u := range good {
		if h, err := skinHash(u); err != nil || h != testSkinHex {
			t.Errorf("%s: %q %v", u, h, err)
		}
	}
	bad := []string{
		"",
		"ftp://textures.minecraft.net/texture/" + testSkinHex,
		"https://textures.minecraft.net.evil.example/texture/" + testSkinHex,
		"https://evil.example/texture/" + testSkinHex,
		"https://textures.minecraft.net@evil.example/texture/" + testSkinHex,
		"https://user@textures.minecraft.net/texture/" + testSkinHex,
		"https://textures.minecraft.net:8443/texture/" + testSkinHex,
		"https://textures.minecraft.net/texture/" + testSkinHex + "?x=1",
		"https://textures.minecraft.net/texture/../" + testSkinHex,
		"https://textures.minecraft.net/texture/" + testSkinHex + "/x",
		"https://textures.minecraft.net/other/" + testSkinHex,
		"https://textures.minecraft.net/texture/NOTHEX",
		"//textures.minecraft.net/texture/" + testSkinHex,
	}
	for _, u := range bad {
		if h, err := skinHash(u); err == nil {
			t.Errorf("%q was accepted as %q", u, h)
		}
	}
}

func TestFaceFromSkinHatAndLegacyRules(t *testing.T) {
	// A legacy 64x32 skin whose hat area is opaque throughout has no hat.
	face, err := faceFromSkin(skinPNG(t, 32, true))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(face))
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(img.At(3, 3)); got != faceRed {
		t.Fatalf("an opaque legacy hat was drawn: %v", got)
	}
	// The same opaque hat on a 64x64 skin is drawn.
	face, err = faceFromSkin(skinPNG(t, 64, true))
	if err != nil {
		t.Fatal(err)
	}
	img, err = png.Decode(bytes.NewReader(face))
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(img.At(3, 3)); got == faceRed {
		t.Fatal("a 64x64 hat was dropped")
	}
	if _, err := faceFromSkin([]byte("\x89PNG\r\n\x1a\ntruncated")); err == nil {
		t.Fatal("a truncated PNG was accepted")
	}
}

func TestMojangFaceMiddlewareServesOnlyTheCachedFace(t *testing.T) {
	f := newMojangFake(t)
	m := f.service(t)
	m.Profile(context.Background(), "Snadrochka")
	if err := os.WriteFile(filepath.Join(m.dir, strings.Repeat("b", 32)+".png"), []byte("not a png"), 0o600); err != nil {
		t.Fatal(err)
	}
	var passed bool
	h := m.FaceMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { passed = true }))
	do := func(method, target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
		return rec
	}
	rec := do(http.MethodGet, "/mojang-face/"+testUUID+".png?v=1")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	if do(http.MethodHead, "/mojang-face/"+testUUID+".png").Code != http.StatusOK {
		t.Fatal("HEAD refused")
	}
	if do(http.MethodPost, "/mojang-face/"+testUUID+".png").Code != http.StatusMethodNotAllowed {
		t.Fatal("POST allowed")
	}
	for _, target := range []string{
		"/mojang-face/lookup.json",
		"/mojang-face/../lookup.json",
		"/mojang-face/" + testUUID + ".png/x",
		"/mojang-face/" + strings.ToUpper(testUUID) + ".png",
		"/mojang-face/" + strings.Repeat("c", 32) + ".png",
		"/mojang-face/" + strings.Repeat("b", 32) + ".png", // cached, but not a PNG
		"/mojang-face/",
	} {
		if rec := do(http.MethodGet, target); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", target, rec.Code)
		}
	}
	if passed {
		t.Fatal("a /mojang-face/ request fell through to the assets")
	}
	do(http.MethodGet, "/index.html")
	if !passed {
		t.Fatal("another path was not handed on")
	}
}

func TestMojangCopyUUIDHandsOverTheLatestFoundProfile(t *testing.T) {
	f := newMojangFake(t)
	m := f.service(t)
	if _, err := m.CopyUUID(); err == nil {
		t.Fatal("copied before any lookup")
	}
	m.Profile(context.Background(), "Snadrochka")
	if got, err := m.CopyUUID(); err != nil || got != testDashed {
		t.Fatalf("%q %v", got, err)
	}
	m.Profile(context.Background(), "no")
	if _, err := m.CopyUUID(); err == nil {
		t.Fatal("an invalid name kept the earlier UUID")
	}
}
