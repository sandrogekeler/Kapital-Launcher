package models

// MapStatus is what the launcher knows about a chapter's web map after one
// reachability check (issue 161): the address the manifest names for it, and
// whether anything answered there. A chapter with no map has an empty URL.
type MapStatus struct {
	// URL is the chapter's manifest map address, or "" when it has none.
	URL string `json:"url"`
	// Reachable is whether an HTTP response of any status came back.
	Reachable bool `json:"reachable"`
	// Reason is a short plain-text cause when it is not reachable: "no map",
	// "timed out" or "no answer". Empty when it is.
	Reason string `json:"reason"`
}
