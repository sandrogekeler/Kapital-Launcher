package services

import (
	"reflect"
	"strings"
	"testing"

	"kapital/backend/models"
)

func toggle(name, prefix string) models.ModToggle {
	return models.ModToggle{Name: name, JarPrefix: prefix}
}

// The bundled manifest's quick switches, with the real jar names each one has
// to match (kapital-packs' .pw.toml files, read 2026-10-04) and one a
// neighbour has to leave alone.
func TestBundledManifestNamesTheQuickToggles(t *testing.T) {
	m, err := ParseManifest(bundledManifest(t))
	if err != nil {
		t.Fatal(err)
	}
	lic, fra := m.Chapters[1], m.Chapters[2]
	if lic.ID != "lichdenstein" || fra.ID != "frangfurd" || len(m.Chapters[0].Pack.Toggles) != 0 {
		t.Fatalf("chapters: %s %s", lic.ID, fra.ID)
	}
	if want := []models.ModToggle{toggle("Distant Horizons", "DistantHorizons-")}; !reflect.DeepEqual(lic.Pack.Toggles, want) {
		t.Fatalf("Lichdenstein: %+v", lic.Pack.Toggles)
	}
	names := []string{}
	for _, tg := range fra.Pack.Toggles {
		names = append(names, tg.Name)
	}
	if want := []string{"Distant Horizons", "Colorwheel", "Colorwheel Patcher", "Create Better FPS"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("Frangfurd: %q", names)
	}
	// Each matches exactly the one jar it is for, whichever version the pack has.
	frangfurd := []string{
		"DistantHorizons-3.3.3-1.21.1-fabric-neoforge.jar",
		"colorwheel-neoforge-1.3.0+mc1.21.1.jar",
		"colorwheel_patcher-neoforge-1.0.5+mc1.21.1.jar",
		"createbetterfps-1.21.1-1.1.5.jar",
		"Jade-1.21.1-NeoForge-15.1.jar",
	}
	for i, tg := range fra.Pack.Toggles {
		var hit []string
		for _, jar := range frangfurd {
			if strings.HasPrefix(jar, tg.JarPrefix) {
				hit = append(hit, jar)
			}
		}
		if !reflect.DeepEqual(hit, frangfurd[i:i+1]) {
			t.Errorf("%s matches %q", tg.Name, hit)
		}
	}
	// And Lichdenstein's own, older Distant Horizons.
	if !strings.HasPrefix("DistantHorizons-2.1.0-a-1.20.6-noForge.jar", lic.Pack.Toggles[0].JarPrefix) {
		t.Error("Lichdenstein's Distant Horizons is not matched")
	}
}

func TestValidateManifestRefusesBadToggles(t *testing.T) {
	long := strings.Repeat("a", 41)
	cases := map[string][]models.ModToggle{
		"an empty name":       {toggle("", "Mod-")},
		"a blank name":        {toggle("  ", "Mod-")},
		"a padded name":       {toggle(" Mod", "Mod-")},
		"a long name":         {toggle(long, "Mod-")},
		"a control character": {toggle("Mo\nd", "Mod-")},
		"an empty prefix":     {toggle("Mod", "")},
		"a short prefix":      {toggle("Mod", "ab")},
		"a path":              {toggle("Mod", "../Mod-")},
		"a separator":         {toggle("Mod", "a/Mod-")},
		"a backslash":         {toggle("Mod", `a\Mod-`)},
		"a wildcard":          {toggle("Mod", "Mod*")},
		"a space":             {toggle("Mod", "Mod x-")},
		"a bracket":           {toggle("Mod", "[Mod]-")},
		"a leading dot":       {toggle("Mod", ".Mod-")},
		"a variable":          {toggle("Mod", "$MOD-")},
		"a long prefix":       {toggle("Mod", strings.Repeat("a", 65))},
		"a name twice":        {toggle("Mod", "Mod-"), toggle("Mod", "Other-")},
		"a prefix twice":      {toggle("Mod", "Mod-"), toggle("Other", "Mod-")},
		"a prefix inside one": {toggle("Mod", "colorwheel-"), toggle("Other", "colorwheel-neoforge-")},
		"a prefix around one": {toggle("Mod", "colorwheel-neoforge-"), toggle("Other", "colorwheel-")},
		"thirteen toggles":    manyToggles(13),
	}
	for name, toggles := range cases {
		t.Run(name, func(t *testing.T) {
			m := validManifest()
			m.Chapters[0].Pack.Toggles = toggles
			if err := ValidateManifest(m); err == nil {
				t.Fatalf("expected %s to be refused", name)
			}
		})
	}
}

func manyToggles(n int) []models.ModToggle {
	out := make([]models.ModToggle, n)
	for i := range out {
		out[i] = toggle(strings.Repeat("n", i+1), "m"+string(rune('a'+i))+"d-")
	}
	return out
}

func TestValidateManifestAcceptsToggles(t *testing.T) {
	m := validManifest()
	// Two Colorwheel mods that share most of a name are told apart by the
	// separator in the prefix.
	m.Chapters[0].Pack.Toggles = []models.ModToggle{
		toggle("Colorwheel", "colorwheel-neoforge-"),
		toggle("Colorwheel Patcher", "colorwheel_patcher-neoforge-"),
		toggle("Mod", "Some.Mod+x-"),
	}
	if err := ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	m.Chapters[0].Pack.Toggles = manyToggles(12)
	if err := ValidateManifest(m); err != nil {
		t.Fatalf("twelve toggles: %v", err)
	}
}

func TestParseManifestReadsToggles(t *testing.T) {
	raw := strings.Replace(string(bundledManifest(t)), `"jarPrefix": "createbetterfps-"`, `"jarPrefix": "createbetterfps-", "command": "calc"`, 1)
	if _, err := ParseManifest([]byte(raw)); err == nil {
		t.Fatal("a toggle with a field this build does not know must be refused")
	}
}
