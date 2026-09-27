package mobile

import "strings"

// SanitizeName strips any path a content provider may have leaked into a
// display name and caps the length.
func SanitizeName(name string) string {
	trimmed := strings.TrimSpace(name)
	if i := strings.LastIndexAny(trimmed, `/\`); i >= 0 {
		trimmed = trimmed[i+1:]
	}
	if len(trimmed) > 120 {
		trimmed = trimmed[:120]
	}
	if strings.TrimSpace(trimmed) == "" {
		return "config"
	}
	return trimmed
}

// OutputFileName is what a decrypted payload is saved or shared under. The name
// the core produced wins; otherwise the source name gains a .txt suffix.
func OutputFileName(sourceName, provided string) string {
	if strings.TrimSpace(provided) != "" {
		return SanitizeName(provided)
	}
	if i := strings.LastIndex(sourceName, "."); i > 0 {
		return SanitizeName(sourceName[:i] + ".txt")
	}
	return SanitizeName(sourceName + ".txt")
}
