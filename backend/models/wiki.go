package models

// WikiPage is one page of the Kapitel Kapital wiki as its lore export lists
// it (#58): enough for the "From the wiki" panel to show a title and a line
// and to open the page. The launcher never renders wiki content; it shows
// these two strings as text and hands the URL to the system browser.
type WikiPage struct {
	// Title is the page's name.
	Title string `json:"title"`
	// Line is the page's opening line, the export's excerpt.
	Line string `json:"line"`
	// URL is the page's absolute address on the wiki's host.
	URL string `json:"url"`
	// Eras names the chapters the page belongs to, by the wiki's era ids,
	// which are the chapter names ("Luxemburg", "Lichdenstein", "Frangfurd").
	Eras []string `json:"eras"`
	// ID is the page's id in the export ("locations/Bellum Castle.md"), which
	// a screenshot's Subject and Related name.
	ID string `json:"id"`
	// Related is the ids of the listed pages the wiki relates to this one, in
	// either direction (#141): the posts a picture of this page may come with.
	Related []string `json:"related"`
}

// WikiShot is one of the wiki's screenshots, downloaded and cached by the
// launcher (#141), for the chapter art and the post that goes with it.
type WikiShot struct {
	// Era is the wiki's era id the picture belongs to, the chapter's name.
	Era string `json:"era"`
	// Src is where the page loads it: the launcher's own /wiki-art/ path,
	// served from the cache, never the wiki's address.
	Src string `json:"src"`
	// Subject is the id of the page the picture shows, "" when none.
	Subject string `json:"subject"`
}
