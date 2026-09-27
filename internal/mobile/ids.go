package mobile

import "strconv"

// parseID turns a data-id attribute back into a job id. A malformed value
// becomes 0, which never matches a job, so a bad tap is a no-op rather than a
// crash.
func parseID(s string) int64 {
	if s == "" {
		return 0
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return id
}
