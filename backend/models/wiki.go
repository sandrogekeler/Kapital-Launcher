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
}
