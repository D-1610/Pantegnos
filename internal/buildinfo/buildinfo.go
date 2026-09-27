// Package buildinfo turns whatever the build injected into a version string
// into something safe to show a user.
//
// Every front-end gets its version from the outside: goreleaser passes
// -X main.version for the CLI, Gradle passes -X main.version for the Android app,
// and the Pages workflow writes web/version.txt. A build that supplies nothing
// must not print an empty badge, and a template engine that fails to substitute
// must not print "{{.Version}}", so all of that collapses to "dev" here.
package buildinfo

import "strings"

// Dev is what every surface shows when the build supplied no version.
const Dev = "dev"

// words are non-empty strings a build can produce that still mean "no version":
// a nil rendered by a template engine, or Go's nil formatting.
var words = map[string]bool{
	"dev":       true,
	"null":      true,
	"nil":       true,
	"<nil>":     true,
	"none":      true,
	"undefined": true,
}

// Normalize returns the version to display, falling back to Dev when the build
// supplied nothing usable. Surrounding whitespace is trimmed.
func Normalize(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || words[strings.ToLower(trimmed)] {
		return Dev
	}
	// A value still carrying a template marker is an unresolved substitution,
	// whatever it happens to be wrapped in.
	if strings.Contains(trimmed, "{{") || strings.Contains(trimmed, "${") {
		return Dev
	}
	return trimmed
}

// Display formats a version for a badge: Dev on its own, otherwise the
// conventional "v" prefix.
func Display(raw string) string {
	version := Normalize(raw)
	if version == Dev {
		return Dev
	}
	return "v" + version
}

// IsDev reports whether this is an unversioned build.
func IsDev(raw string) bool { return Normalize(raw) == Dev }
