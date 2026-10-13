package videos

import "strings"

func SanitizeSnippet(value string) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	if cleaned == "" {
		return "(no description)"
	}
	const maxLen = 180
	if len(cleaned) <= maxLen {
		return cleaned
	}
	return cleaned[:maxLen-3] + "..."
}
