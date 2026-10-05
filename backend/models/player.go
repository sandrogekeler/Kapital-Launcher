package models

// Player lookup outcomes (ADR-13).
const (
	// PlayerFound: Mojang knows the name. Name and UUID are set.
	PlayerFound = "found"
	// PlayerNotFound: Mojang answered and no profile has the name.
	PlayerNotFound = "not_found"
	// PlayerUnknown: Mojang could not be asked, or answered nothing usable
	// (offline, a timeout, a rate limit). The page says nothing about it.
	PlayerUnknown = "unknown"
	// PlayerInvalid: the name cannot be a Minecraft name, so nothing was asked.
	PlayerInvalid = "invalid"
)

// PlayerProfile is what the launcher knows about a player from Mojang's public
// profile endpoints (ADR-13): no sign-in, no token. Only the status is set for
// every outcome but found.
type PlayerProfile struct {
	// Name is Mojang's own spelling of the profile name ("" unless found).
	Name string `json:"name"`
	// UUID is the profile's id with dashes, "" unless found.
	UUID string `json:"uuid"`
	// FaceSrc is where the page loads the player's face: the launcher's own
	// /mojang-face/ path, served from the cache, never Mojang's address. ""
	// when the face could not be fetched.
	FaceSrc string `json:"faceSrc"`
	// Status is one of the Player* constants.
	Status string `json:"status"`
}
