package splashhost

import (
	"io/fs"
	"path"
	"strings"
)

// mimeTypes is every type the card's build holds and the hosts serve. A file
// with another extension is refused rather than guessed at.
var mimeTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".woff2": "font/woff2",
	".png":   "image/png",
	".webp":  "image/webp",
	".svg":   "image/svg+xml",
	".json":  "application/json",
	".mp4":   "video/mp4",
}

// AssetsFrom serves files from root, the embedded frontend build (the
// "frontend/dist" of main.go's embed, as an fs.FS), for Page.Assets. A path is
// served only when it is a clean relative path inside root (no "..", no
// backslash, no NUL, no leading slash), names a regular file and has an
// extension in mimeTypes; everything else is ok=false. A nil root serves
// nothing.
func AssetsFrom(root fs.FS) func(string) ([]byte, string, bool) {
	return func(name string) ([]byte, string, bool) {
		if root == nil || !fs.ValidPath(name) || name == "." || strings.ContainsAny(name, "\\\x00") {
			return nil, "", false
		}
		mime, known := mimeTypes[strings.ToLower(path.Ext(name))]
		if !known {
			return nil, "", false
		}
		info, err := fs.Stat(root, name)
		if err != nil || !info.Mode().IsRegular() {
			return nil, "", false
		}
		body, err := fs.ReadFile(root, name)
		if err != nil {
			return nil, "", false
		}
		return body, mime, true
	}
}
