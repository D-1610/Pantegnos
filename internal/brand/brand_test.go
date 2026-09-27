package brand

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot walks up to the module root so the tests can reach files outside the
// package directory, such as web/ and android/.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod")
		}
		dir = parent
	}
}

var (
	svgPathData    = regexp.MustCompile(`\sd="([^"]+)"`)
	vectorPathData = regexp.MustCompile(`android:pathData="([^"]+)"`)
)

func paths(t *testing.T, file string, re *regexp.Regexp) []string {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	var out []string
	for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
		out = append(out, normalise(m[1]))
	}
	if len(out) == 0 {
		t.Fatalf("no path data found in %s", file)
	}
	return out
}

// normalise collapses whitespace so a file that merely indents differently still
// compares equal.
func normalise(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, v := range a {
		seen[v]++
	}
	for _, v := range b {
		seen[v]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

// TestAndroidVectorMatches guards the launcher icon against the canonical SVG.
// The drawable has to be a VectorDrawable, so it cannot embed the file, but its
// geometry must not drift.
func TestAndroidVectorMatches(t *testing.T) {
	root := repoRoot(t)
	svg := paths(t, filepath.Join(root, "internal", "brand", "logo.svg"), svgPathData)
	vec := paths(t, filepath.Join(root, "android", "app", "src", "main", "res", "drawable", "ic_launcher_foreground.xml"), vectorPathData)

	if !sameSet(svg, vec) {
		t.Errorf("the launcher drawable has drifted from logo.svg\n svg: %v\n vec: %v", svg, vec)
	}
}

// TestWebCopyMatches stops web/logo.svg from lagging behind the canonical file.
func TestWebCopyMatches(t *testing.T) {
	root := repoRoot(t)
	canonical, err := os.ReadFile(filepath.Join(root, "internal", "brand", "logo.svg"))
	if err != nil {
		t.Fatal(err)
	}
	web, err := os.ReadFile(filepath.Join(root, "web", "logo.svg"))
	if err != nil {
		t.Fatalf("web/logo.svg is missing: %v", err)
	}
	if !bytes.Equal(canonical, web) {
		t.Error("web/logo.svg differs from internal/brand/logo.svg; copy it again")
	}
}

// TestFaviconsPresent checks the raster forms the web page links to exist.
func TestFaviconsPresent(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"favicon-32.png", "favicon-192.png"} {
		if _, err := os.Stat(filepath.Join(root, "web", name)); err != nil {
			t.Errorf("web/%s is missing", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "brand", "icon.ico")); err != nil {
		t.Error("internal/brand/icon.ico is missing; run: go run ./tools/icon")
	}
}

// TestLogoIsSelfContained guards against the same class of bug the stylesheet
// had: a stray backtick in a Go raw string would silently truncate the file.
func TestLogoIsSelfContained(t *testing.T) {
	if !bytes.HasSuffix([]byte(LogoSVG()), []byte("</svg>\n")) {
		t.Error("logo.svg does not end with </svg>; check for an unterminated raw string")
	}
	if len(LogoSolidSVG()) == 0 {
		t.Error("logo-solid.svg is empty")
	}
}
