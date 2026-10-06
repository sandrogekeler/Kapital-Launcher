package services

import (
	"encoding/binary"
	"errors"
	"fmt"
	"path"
	"strings"
)

// Prism's macOS zip keeps part of its signature outside the files themselves.
// A file in a bundle's Contents/MacOS that is not Mach-O (Prism's three jars)
// carries its code signature in extended attributes, com.apple.cs.*, and a zip
// made on a Mac stores a file's attributes as an AppleDouble file of their own,
// __MACOSX/<dir>/._<name>. A plain unzip writes those as files and leaves the
// jars unsigned, so codesign refuses the bundle ("code object is not signed at
// all", seen on a macOS runner with Prism 11.1.1). unzipBounded therefore reads
// the AppleDouble entries instead of writing them and, on macOS, puts back the
// code signature attributes of the file each names (managedprism_xattr_darwin.go).
//
// The format is Apple's AppleDouble version 2 with the extended attributes
// inside its Finder Info entry, under an "ATTR" header, as macOS's copyfile
// writes it (copyfile.c, apple_double_header and attr_header).

// appleDoubleDir is the folder a Mac zip keeps AppleDouble files in.
const appleDoubleDir = "__MACOSX"

// maxAppleDouble bounds one AppleDouble entry: Prism's are under 10 KiB.
const maxAppleDouble = 1 << 20

// appleDoubleTarget is the file an AppleDouble entry describes, relative to the
// archive's root, from its slash-separated name: "__MACOSX/a/b/._c" describes
// "a/b/c". ok is false for a name that is not an AppleDouble file, including the
// folders under __MACOSX.
func appleDoubleTarget(name string) (string, bool) {
	rest, found := strings.CutPrefix(name, appleDoubleDir+"/")
	if !found {
		return "", false
	}
	dir, base := path.Split(rest)
	file, found := strings.CutPrefix(base, "._")
	if !found || file == "" {
		return "", false
	}
	return dir + file, true
}

// isAppleDoubleEntry is whether a zip entry belongs to the AppleDouble folder,
// the folder itself included.
func isAppleDoubleEntry(name string) bool {
	return name == appleDoubleDir || name == appleDoubleDir+"/" || strings.HasPrefix(name, appleDoubleDir+"/")
}

var errAppleDouble = errors.New("not an AppleDouble file with extended attributes")

// parseAppleDoubleXattrs reads the extended attributes out of an AppleDouble
// file. A file without them (only Finder Info, or a resource fork) has none, and
// that is not an error. Every offset is checked against the data.
func parseAppleDoubleXattrs(data []byte) (map[string][]byte, error) {
	be := binary.BigEndian
	if len(data) < 26 || be.Uint32(data[0:4]) != 0x00051607 || be.Uint32(data[4:8]) != 0x00020000 {
		return nil, errAppleDouble
	}
	entries := int(be.Uint16(data[24:26]))
	if 26+12*entries > len(data) {
		return nil, errAppleDouble
	}
	attrs := map[string][]byte{}
	for i := range entries {
		e := data[26+12*i : 38+12*i]
		id, off, length := be.Uint32(e[0:4]), int(be.Uint32(e[4:8])), int(be.Uint32(e[8:12]))
		if id != 9 { // Finder Info, where macOS keeps the attributes
			continue
		}
		if off < 0 || length < 0 || off > len(data) || length > len(data)-off {
			return nil, errAppleDouble
		}
		// 32 bytes of Finder Info and two of padding, then the ATTR header.
		h := off + 34
		if length < 34+36 || string(data[h:h+4]) != "ATTR" {
			continue
		}
		count := int(be.Uint16(data[h+34 : h+36]))
		p := h + 36
		for range count {
			if p+11 > len(data) {
				return nil, errAppleDouble
			}
			aoff, alen, nameLen := int(be.Uint32(data[p:p+4])), int(be.Uint32(data[p+4:p+8])), int(data[p+10])
			if nameLen == 0 || p+11+nameLen > len(data) || aoff < 0 || alen < 0 || aoff > len(data) || alen > len(data)-aoff {
				return nil, errAppleDouble
			}
			name := strings.TrimRight(string(data[p+11:p+11+nameLen]), "\x00")
			if name == "" || strings.ContainsRune(name, 0) {
				return nil, errAppleDouble
			}
			attrs[name] = data[aoff : aoff+alen]
			p = (p + 11 + nameLen + 3) &^ 3 // each entry starts on four bytes
		}
	}
	return attrs, nil
}

// codeSignatureAttrs keeps the attributes a code signature lives in and drops
// every other, quarantine and Finder data included: the launcher restores what
// codesign checks and nothing it was not asked to carry.
func codeSignatureAttrs(attrs map[string][]byte) map[string][]byte {
	out := map[string][]byte{}
	for name, value := range attrs {
		if strings.HasPrefix(name, "com.apple.cs.") {
			out[name] = value
		}
	}
	return out
}

// appleDoubleEntry is one AppleDouble file read from the archive, kept until the
// file it names has been written.
type appleDoubleEntry struct {
	target string // the file it describes, a cleaned native path in the archive
	attrs  map[string][]byte
}

func (e appleDoubleEntry) String() string {
	return fmt.Sprintf("%s (%d attributes)", e.target, len(e.attrs))
}
