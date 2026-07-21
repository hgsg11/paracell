package domain

import "strings"

// SafeResourceName converts arbitrary input into a name that can safely be
// shared by filesystem paths, tmux sessions, and Docker resources.
func SafeResourceName(value string, fallback string) string {
	result := strings.Trim(safeResourceName(value), "._-")
	if result != "" {
		return result
	}

	result = strings.Trim(safeResourceName(fallback), "._-")
	if result != "" {
		return result
	}
	return "cell"
}

func safeResourceName(value string) string {
	var name strings.Builder
	replace := false
	for _, r := range value {
		if isSafeResourceRune(r) {
			if replace && name.Len() > 0 {
				name.WriteByte('-')
			}
			name.WriteRune(r)
			replace = false
			continue
		}
		replace = true
	}

	return name.String()
}

func isSafeResourceRune(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' ||
		r == '.' || r == '_' || r == '-'
}
