package api

import "strings"

func normalizeLanguage(s string) string {
	if strings.HasPrefix(s, "ru") {
		return "ru"
	}
	return "en"
}
