package services

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"

	"kapital/backend/models"
)

// The wiki's page pictures and the daily draw (issue 172). Besides its
// screenshots, the lore export lists each page's pictures as pages[].images,
// site paths under /vault/images/ in the order the page embeds them (an export
// of an older wiki build has no such field, and the launcher then has the
// screenshots alone). A chapter's pool is its era's screenshots plus its era's
// pages' pictures, and the launcher downloads only a set number of them: it
// draws up to the player's number (5, 10, 20, or all) once a day, never more
// than one picture of a page before every page with pictures left has had one.

const (
	// maxWikiPictures bounds how many page pictures the export may list, one
	// entry per picture and era.
	maxWikiPictures = 1000
	// wikiPicturesDir is the cache folder of the page pictures, and the world
	// no screenshot may name (parseWikiShots), so a name cannot collide.
	wikiPicturesDir = "pages"
	// wikiPicturesSite is where the wiki serves a page's pictures.
	wikiPicturesSite = "/vault/images/"
	// defaultArtBytes is what a picture is taken to weigh until the cache has
	// some of its own: the wiki's are about 100 KB.
	defaultArtBytes = 100 << 10
)

// wikiPictureFile is a page picture's file name: plain characters, an image
// extension, no separators (a dot segment is refused apart, as for screenshots).
var wikiPictureFile = regexp.MustCompile(`^[a-z0-9][A-Za-z0-9._-]*\.(?:webp|png|jpe?g)$`)

// wikiPicturePath is a page picture's site path.
var wikiPicturePath = regexp.MustCompile(`^` + regexp.QuoteMeta(wikiPicturesSite) + `([a-z0-9][A-Za-z0-9._-]*\.(?:webp|png|jpe?g))$`)

// parseWikiPictures keeps the pictures the export's listable pages embed, one
// per era of the page, in the export's order. A page the panel does not pick
// from (an index, the timeline, a stub), one without an era and a path that
// fails the shape are dropped on their own; so is an images field that is not
// a list of strings. The page's id is the picture's subject.
func parseWikiPictures(raw []byte) []wikiShot {
	var doc struct {
		Pages []struct {
			ID     string          `json:"id"`
			Type   string          `json:"type"`
			Era    []string        `json:"era"`
			Status string          `json:"status"`
			Images json.RawMessage `json:"images"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	var out []wikiShot
	for _, p := range doc.Pages {
		if slices.Contains(wikiSkipTypes, p.Type) || slices.Contains(wikiSkipStatuses, p.Status) {
			continue
		}
		var items []json.RawMessage
		if len(p.Images) == 0 || json.Unmarshal(p.Images, &items) != nil {
			continue
		}
		subject := strings.TrimSpace(p.ID)
		if len(subject) > maxWikiID {
			subject = ""
		}
		var eras []string
		for _, e := range p.Era {
			if e = strings.TrimSpace(e); e != "" && len(e) <= maxWikiEra && !slices.Contains(eras, e) {
				eras = append(eras, e)
			}
		}
		var files []string
		for _, item := range items {
			var img string
			if json.Unmarshal(item, &img) != nil {
				continue
			}
			m := wikiPicturePath.FindStringSubmatch(img)
			if m != nil && !strings.Contains(m[1], "..") && !slices.Contains(files, m[1]) {
				files = append(files, m[1])
			}
		}
		for _, era := range eras {
			for _, f := range files {
				if len(out) == maxWikiPictures {
					return out
				}
				out = append(out, wikiShot{era: era, world: wikiPicturesDir, file: f, subject: subject})
			}
		}
	}
	return out
}

// wikiPool is the picture pool of one era, in the export's order: the era's
// screenshots, then its pages' pictures.
type wikiPool struct {
	era  string
	pics []wikiShot
}

// wikiPools builds the pool of each era the export names, in the order each
// era first appears. With chapters known, only their eras are pooled, in the
// manifest's order, and an era with no picture still has an (empty) pool.
func (w *WikiService) wikiPools(raw []byte) []wikiPool {
	var pools []wikiPool
	at := func(era string) *wikiPool {
		for i := range pools {
			if pools[i].era == era {
				return &pools[i]
			}
		}
		pools = append(pools, wikiPool{era: era})
		return &pools[len(pools)-1]
	}
	for _, c := range w.chapters {
		at(c.Name)
	}
	for _, s := range append(parseWikiShots(raw), parseWikiPictures(raw)...) {
		if len(w.chapters) > 0 && !slices.ContainsFunc(w.chapters, func(c models.Chapter) bool { return c.Name == s.era }) {
			continue
		}
		p := at(s.era)
		// A picture two pages embed is in the pool once, under the first page.
		if slices.ContainsFunc(p.pics, func(o wikiShot) bool { return o.rel() == s.rel() }) {
			continue
		}
		p.pics = append(p.pics, s)
	}
	return pools
}

// seedKey is what a chapter's draw is seeded with: the local date and the
// chapter's id, so a set holds through a day and is another on the next. An era
// no chapter of the manifest has is keyed by its name.
func (w *WikiService) seedKey(date, era string) string {
	for _, c := range w.chapters {
		if c.Name == era {
			return date + "|" + c.ID
		}
	}
	return date + "|" + era
}

// drawPictures picks up to n of the pool, 0 meaning all of it, which a pool of
// no more than n is as well. Pictures are grouped by the page they show (a
// picture with no page is a group of its own). Each round goes through the
// groups that still have pictures in a random order and takes a random one of
// each, so no page gives a second picture before every other page with
// pictures left has given one, and a round the number ends takes a random few of
// the groups. The random source is seeded by seed alone, so the same pool, number
// and seed draw the same pictures.
func drawPictures(pool []wikiShot, n int, seed string) []wikiShot {
	if n <= 0 || n >= len(pool) {
		return slices.Clone(pool)
	}
	var groups [][]wikiShot
	at := map[string]int{}
	for _, s := range pool {
		key := s.subject
		if key == "" {
			key = "\x00" + s.world + "/" + s.file
		}
		i, ok := at[key]
		if !ok {
			i = len(groups)
			at[key] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], s)
	}
	sum := sha256.Sum256([]byte(seed))
	rng := rand.New(rand.NewPCG(binary.BigEndian.Uint64(sum[:8]), binary.BigEndian.Uint64(sum[8:16])))
	drawn := make([]wikiShot, 0, n)
	for len(drawn) < n {
		for _, g := range rng.Perm(len(groups)) {
			if len(drawn) == n {
				break
			}
			items := groups[g]
			if len(items) == 0 {
				continue
			}
			j := rng.IntN(len(items))
			drawn = append(drawn, items[j])
			groups[g] = slices.Delete(items, j, j+1)
		}
	}
	return drawn
}

// WikiPictureChoices are the numbers of pictures per chapter the settings
// offer, 0 being all of them.
var WikiPictureChoices = []int{5, 10, 20, 0}

// DefaultWikiPictures is the number of pictures per chapter when settings name
// none.
const DefaultWikiPictures = 10

// WikiPictures is the number of pictures a chapter's set holds: the stored
// choice, DefaultWikiPictures when there is none, and 0 for all of them.
func WikiPictures(s models.AppSettings) int {
	if s.WikiPictures == nil {
		return DefaultWikiPictures
	}
	return *s.WikiPictures
}
