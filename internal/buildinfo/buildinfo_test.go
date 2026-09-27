package buildinfo

import "testing"

func TestNormalizeFallsBackToDev(t *testing.T) {
	for _, raw := range []string{
		"", "   ", "\n\t",
		"dev", "DEV", "Dev",
		"null", "NULL", "nil", "<nil>", "none", "undefined",
		"{{.Version}}", "${VERSION}",
	} {
		if got := Normalize(raw); got != Dev {
			t.Errorf("Normalize(%q) = %q, want %q", raw, got, Dev)
		}
	}
}

func TestNormalizeKeepsRealVersions(t *testing.T) {
	cases := map[string]string{
		"1.0.0":        "1.0.0",
		"  1.0.0  ":    "1.0.0",
		"v1.2.3":       "v1.2.3",
		"1.2.3-rc1":    "1.2.3-rc1",
		"2026.05.17":   "2026.05.17",
		"1.0.0-native": "1.0.0-native",
		"devtools":     "devtools",
	}
	for raw, want := range cases {
		if got := Normalize(raw); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestDisplay(t *testing.T) {
	cases := map[string]string{
		"1.0.0":    "v1.0.0",
		" 1.0.0 ":  "v1.0.0",
		"v1.0.0":   "vv1.0.0", // a pre-prefixed version is shown verbatim
		"":         "dev",
		"null":     "dev",
		"{{.Ver}}": "dev",
	}
	for raw, want := range cases {
		if got := Display(raw); got != want {
			t.Errorf("Display(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestIsDev(t *testing.T) {
	if !IsDev("") || !IsDev("null") || !IsDev("dev") {
		t.Error("IsDev should be true for every placeholder")
	}
	if IsDev("1.0.0") {
		t.Error("IsDev should be false for a real version")
	}
}
