package mobile

import (
	"embed"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

//go:embed locale/en.json locale/fa.json
var localeFS embed.FS

// catalog maps a locale code to its translated strings.
type catalog map[string]string

var (
	catalogs = map[string]catalog{}
	active   = "en"
)

func init() {
	for _, code := range []string{"en", "fa"} {
		raw, err := localeFS.ReadFile("locale/" + code + ".json")
		if err != nil {
			continue
		}
		var c catalog
		if err := json.Unmarshal(raw, &c); err != nil {
			continue
		}
		catalogs[code] = c
	}
}

// SetLocale switches the active catalogue. An unknown code is ignored.
func SetLocale(code string) {
	if _, ok := catalogs[code]; ok {
		active = code
	}
}

// Locale returns the active language code.
func Locale() string { return active }

// IsRTL reports whether the active language is written right to left.
func IsRTL() bool { return active == "fa" }

// t translates a key, formatting any %s placeholders with args. A missing
// translation falls back to English and then to the key itself, so a gap shows
// up as an obvious identifier rather than an empty label.
func t(key string, args ...any) string {
	text, ok := catalogs[active][key]
	if !ok {
		if fallback, ok := catalogs["en"][key]; ok {
			text = fallback
		} else {
			return key
		}
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

// localeKeys exposes the key sets so tests can prove every locale is complete.
func localeKeys() map[string][]string {
	out := map[string][]string{}
	for code, c := range catalogs {
		out[code] = slices.Sorted(maps.Keys(c))
	}
	return out
}
