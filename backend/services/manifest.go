package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"kapital/backend/design"
	"kapital/backend/models"
)

// ManifestVersion is the only shape this build understands. A newer manifest
// is refused rather than half-read.
const ManifestVersion = 1

// AllowedManifestHosts is the fixed allowlist a manifest URL must be on. A
// manifest that names another host is refused whole, because a URL in it is
// something the app will fetch or open (agent_docs/SECURITY_CHECKLIST.md, S3).
var AllowedManifestHosts = []string{
	"kapitel-kapital.pages.dev",
	// The packs: kapital-packs served from a Worker with static assets, only
	// its chapter folders (#25; kapital-packs' README, Hosting).
	"kapital-packs.alessandrogekeler.workers.dev",
	"github.com",
	"raw.githubusercontent.com",
	"modrinth.com",
	"cdn.modrinth.com",
}

var (
	chapterIDPattern  = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)
	instanceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	numberPattern     = regexp.MustCompile(`^[0-9]{2}$`)
	wikiPathPattern   = regexp.MustCompile(`^/[^\s]*$`)
	chapterStates     = []string{"released", "development", "planned"}
	packTypes         = []string{"modpack", "client-visuals"}
	// A pack.toml URL ends up on the pre-launch command line, where Prism
	// expands $ and splits on spaces and double quotes; none of those, nor a
	// backslash or a # (a comment in Prism's older instance.cfg format), may
	// appear in it.
	commandSafeURL = regexp.MustCompile(`^https?://[A-Za-z0-9._~:/?&=%+@!,;()*'-]+$`)
)

// ParseManifest decodes and validates a manifest. Unknown fields are an error:
// a field this build does not know is either a typo or a newer shape, and in
// both cases silently dropping it is the wrong answer.
func ParseManifest(data []byte) (models.Manifest, error) {
	var m models.Manifest
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return models.Manifest{}, fmt.Errorf("manifest: %w", err)
	}
	if err := ValidateManifest(m); err != nil {
		return models.Manifest{}, err
	}
	return m, nil
}

// ValidateManifest is the Go side of design/launcher.schema.json, plus the
// checks a schema cannot express: chapter ids are unique and known to the
// token set, and every URL is https on an allowlisted host.
func ValidateManifest(m models.Manifest) error {
	if m.Version != ManifestVersion {
		return fmt.Errorf("manifest: version %d, this build understands %d", m.Version, ManifestVersion)
	}
	if err := checkURL("wiki.baseUrl", m.Wiki.BaseURL); err != nil {
		return err
	}
	if len(m.Chapters) == 0 {
		return fmt.Errorf("manifest: no chapters")
	}
	seen := map[string]bool{}
	for i, c := range m.Chapters {
		where := fmt.Sprintf("manifest: chapters[%d]", i)
		if !chapterIDPattern.MatchString(c.ID) {
			return fmt.Errorf("%s: id %q is not lowercase letters, digits and hyphens", where, c.ID)
		}
		if seen[c.ID] {
			return fmt.Errorf("%s: id %q appears twice", where, c.ID)
		}
		seen[c.ID] = true
		if !slices.Contains(design.ChapterIDs, c.ID) {
			return fmt.Errorf("%s: id %q has no accent in design/tokens.json", where, c.ID)
		}
		if !numberPattern.MatchString(c.Number) {
			return fmt.Errorf("%s: number %q is not two digits", where, c.Number)
		}
		for name, v := range map[string]string{"name": c.Name, "era": c.Era, "kind": c.Kind, "blurb": c.Blurb} {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("%s: %s is empty", where, name)
			}
		}
		// The name is written into the chapter's instance.cfg (#22).
		if strings.ContainsFunc(c.Name, unicode.IsControl) {
			return fmt.Errorf("%s: name carries a control character", where)
		}
		if !slices.Contains(chapterStates, c.State) {
			return fmt.Errorf("%s: state %q is not one of %v", where, c.State, chapterStates)
		}
		if !instanceIDPattern.MatchString(c.Instance.ID) {
			return fmt.Errorf("%s: instance id %q could name a path", where, c.Instance.ID)
		}
		if !slices.Contains(packTypes, c.Pack.Type) {
			return fmt.Errorf("%s: pack type %q is not one of %v", where, c.Pack.Type, packTypes)
		}
		if strings.TrimSpace(c.Pack.Loader) == "" || strings.TrimSpace(c.Pack.Minecraft) == "" {
			return fmt.Errorf("%s: pack loader and minecraft must be set (use [PLACEHOLDER])", where)
		}
		if c.Pack.Packwiz != nil {
			if err := checkURL(where+".pack.packwiz", *c.Pack.Packwiz); err != nil {
				return err
			}
			if !commandSafeURL.MatchString(*c.Pack.Packwiz) {
				return fmt.Errorf("%s: pack.packwiz %q carries a character a command line would read", where, *c.Pack.Packwiz)
			}
		}
		if c.Pack.JVM != nil {
			preset, ok := jvmPresets[*c.Pack.JVM]
			if !ok {
				return fmt.Errorf("%s: jvm preset %q is not one the launcher knows", where, *c.Pack.JVM)
			}
			if !minecraftAtLeast(c.Pack.Minecraft, preset.minMinecraft) {
				return fmt.Errorf("%s: jvm preset %q needs Minecraft %s or later", where, *c.Pack.JVM, preset.minMinecraft)
			}
		}
		if err := validateToggles(where, c.Pack.Toggles); err != nil {
			return err
		}
		if c.Pack.Mrpack != nil {
			if err := checkURL(where+".pack.mrpack", *c.Pack.Mrpack); err != nil {
				return err
			}
		}
		if err := validateServer(where, c.Server); err != nil {
			return err
		}
		if strings.TrimSpace(c.Wiki.Title) == "" || strings.TrimSpace(c.Wiki.Line) == "" {
			return fmt.Errorf("%s: wiki teaser is incomplete", where)
		}
		if !wikiPathPattern.MatchString(c.Wiki.Path) {
			return fmt.Errorf("%s: wiki path %q must be absolute and carry no whitespace", where, c.Wiki.Path)
		}
	}
	return nil
}

// maxToggles is how many quick switches a chapter may name.
const maxToggles = 12

// modTogglePrefixShape is what a manifest may name as the start of a jar: the
// part of a file name before the version, "DistantHorizons-". Narrower than a
// jar name, with no space or bracket, and long enough not to match a stranger.
var modTogglePrefixShape = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{2,63}$`)

// validateToggles holds a chapter's quick mod switches to what a manifest may
// say about a mod: a name to show and the start of its jar's file name, in a
// shape with no separator, space or bracket (modTogglePrefixShape). Names are
// unique, prefixes are unique, and no prefix starts another, so a jar is never
// two switches. A prefix names jars and never runs one: it is only matched
// against the file names in the instance's mods folder.
func validateToggles(where string, toggles []models.ModToggle) error {
	if len(toggles) > maxToggles {
		return fmt.Errorf("%s: pack.toggles has %d entries, the limit is %d", where, len(toggles), maxToggles)
	}
	for i, t := range toggles {
		at := fmt.Sprintf("%s.pack.toggles[%d]", where, i)
		name := strings.TrimSpace(t.Name)
		if name == "" || name != t.Name || len(name) > 40 || strings.ContainsFunc(name, unicode.IsControl) {
			return fmt.Errorf("%s: name %q is not a short plain name", at, t.Name)
		}
		if !modTogglePrefixShape.MatchString(t.JarPrefix) {
			return fmt.Errorf("%s: jarPrefix %q is not the start of a jar file name", at, t.JarPrefix)
		}
		for j, other := range toggles[:i] {
			switch {
			case other.Name == t.Name:
				return fmt.Errorf("%s: name %q is used by toggles[%d] too", at, t.Name, j)
			case strings.HasPrefix(t.JarPrefix, other.JarPrefix) || strings.HasPrefix(other.JarPrefix, t.JarPrefix):
				return fmt.Errorf("%s: jarPrefix %q overlaps toggles[%d]'s %q", at, t.JarPrefix, j, other.JarPrefix)
			}
		}
	}
	return nil
}

// checkURL accepts only https on an allowlisted host, with no credentials in
// the URL. It is the one gate every manifest URL passes through.
func checkURL(field, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("manifest: %s: %w", field, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("manifest: %s: %q is not https", field, raw)
	}
	if u.User != nil {
		return fmt.Errorf("manifest: %s: %q carries credentials", field, raw)
	}
	host := strings.ToLower(u.Hostname())
	if !slices.Contains(AllowedManifestHosts, host) {
		return fmt.Errorf("manifest: %s: host %q is not on the allowlist", field, host)
	}
	return nil
}

// loopbackHosts are the only hosts a local pack override may name (#41).
var loopbackHosts = []string{"localhost", "127.0.0.1", "::1"}

// CheckLocalPackURL is the rule for a developer's pack override: packwiz
// serve's address on this machine, e.g. http://localhost:8080/pack.toml. It
// lands on the same pre-launch command line as a manifest URL, so the same
// character rules hold, and it can never name another machine.
func CheckLocalPackURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("pack override: %w", err)
	}
	switch {
	case u.Scheme != "http":
		return fmt.Errorf("pack override: %q is not http", raw)
	case !slices.Contains(loopbackHosts, strings.ToLower(u.Hostname())):
		return fmt.Errorf("pack override: %q is not on this machine (localhost, 127.0.0.1 or [::1])", raw)
	case u.Port() == "":
		return fmt.Errorf("pack override: %q names no port", raw)
	case u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Contains(raw, "#"):
		return fmt.Errorf("pack override: %q carries user info, a query or a fragment", raw)
	case !strings.HasSuffix(u.Path, "/pack.toml"):
		return fmt.Errorf("pack override: %q does not end in /pack.toml", raw)
	// [::1]'s brackets are the one addition: Prism splits the command on
	// spaces and quotes only, and the value is quoted in instance.cfg.
	case !commandSafeURL.MatchString(strings.Replace(raw, "//[::1]:", "//localhost:", 1)):
		return fmt.Errorf("pack override: %q carries a character a command line would read", raw)
	}
	return nil
}

// IsLocalPackURL reports whether a pack URL passes CheckLocalPackURL.
func IsLocalPackURL(raw string) bool { return CheckLocalPackURL(raw) == nil }

// WikiURL joins a chapter's wiki path onto the manifest's base URL. Both were
// validated on load, so the result is always https on the wiki's host.
func WikiURL(m models.Manifest, c models.Chapter) string {
	return strings.TrimRight(m.Wiki.BaseURL, "/") + c.Wiki.Path
}
