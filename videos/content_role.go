package videos

import (
	"regexp"
	"strconv"
	"strings"
)

type ContentRole string

const (
	ContentRoleFullGame          ContentRole = "full_game"
	ContentRoleOfficialHighlight ContentRole = "official_highlight"
	ContentRoleHighlight         ContentRole = "highlight"
	ContentRoleRadio             ContentRole = "radio"
	ContentRoleGameFilm          ContentRole = "game_film"
	ContentRolePart              ContentRole = "part"
	ContentRoleStory             ContentRole = "story"
	ContentRoleUnknown           ContentRole = "unknown"
)

type PartInfo struct {
	Part  int
	Total int
}

var (
	preseasonWeekRe = regexp.MustCompile(`(?i)\b(?:preseason|pretemporada|pres)\s*(?:week|semana|game|#)?\s*0*(\d{1,2})\b`)
	spanishWeekRe   = regexp.MustCompile(`(?i)\bsemana\s*0*(\d{1,2})\b`)
	gameNumberRe    = regexp.MustCompile(`(?i)\bgame\s*0*(\d{1,2})\b`)
	partOfRe        = regexp.MustCompile(`(?i)\b(?:pt|part)\.?\s*#?\s*(\d{1,2})(?:\s*(?:of|/)\s*(\d{1,2}))?\b`)
	halfRe          = regexp.MustCompile(`(?i)\b(1st|first|2nd|second)\s+half\b`)
	hashPartRe      = regexp.MustCompile(`(?i)(?:^|\s)#\s*(\d{1,2})(?:\s|$)`)
	partStripRe     = regexp.MustCompile(`(?i)\b(?:pt|part)\.?\s*#?\s*\d{1,2}(?:\s*(?:of|/)\s*\d{1,2})?\b`)
	halfStripRe     = regexp.MustCompile(`(?i)\b(?:1st|first|2nd|second)\s+half\b`)
	hashPartStripRe = regexp.MustCompile(`(?i)(?:^|\s)#\s*\d{1,2}(?:\s|$)`)
)

func DetectGameWeekMatch(text string, game GameMetadata) DateMatchType {
	week := strings.TrimSpace(game.Week)
	if week == "" {
		return DateMatchNone
	}
	norm := NormalizeText(text)
	if isPreseasonWeek(week) {
		want, ok := parsePreseasonWeekNumber(week)
		if !ok {
			return DateMatchNone
		}
		weeks := extractPreseasonWeeks(text)
		if len(weeks) == 0 && (containsNormalizedTerm(norm, "preseason") || containsNormalizedTerm(norm, "pretemporada")) {
			for _, weekText := range append(countWeekMarkers(text), extractSpanishWeeks(text)...) {
				if got, ok := parseWeekNumber(weekText); ok {
					weeks = append(weeks, got)
				}
			}
		}
		if len(weeks) == 0 {
			if containsNormalizedTerm(norm, "preseason") || containsNormalizedTerm(norm, "pretemporada") {
				return DateMatchNone
			}
			if len(countWeekMarkers(text)) > 0 || len(extractSpanishWeeks(text)) > 0 {
				return DateMatchWrong
			}
			return DateMatchNone
		}
		for _, got := range weeks {
			if got == want {
				return DateMatchFull
			}
		}
		return DateMatchWrong
	}

	want, ok := parseWeekNumber(week)
	if !ok {
		if isPlayoffWeek(week) {
			return legacyWeekMatch(GameRef{Week: week}, text)
		}
		return DateMatchNone
	}
	if containsNormalizedTerm(norm, "preseason") || containsNormalizedTerm(norm, "pretemporada") {
		return DateMatchWrong
	}
	weeks := append(countWeekMarkers(text), extractSpanishWeeks(text)...)
	weeks = append(weeks, extractGameNumberWeeks(text)...)
	if len(weeks) == 0 {
		return DateMatchNone
	}
	for _, weekText := range weeks {
		got, ok := parseWeekNumber(weekText)
		if ok && got == want {
			return DateMatchFull
		}
	}
	return DateMatchWrong
}

func isPreseasonWeek(week string) bool {
	week = strings.TrimSpace(strings.ToUpper(week))
	if strings.HasPrefix(week, "PRE") {
		return true
	}
	if strings.HasPrefix(week, "PS") {
		_, ok := parseWeekNumber(strings.TrimSpace(strings.TrimPrefix(week, "PS")))
		return ok
	}
	if strings.HasPrefix(week, "P") {
		_, ok := parseWeekNumber(strings.TrimSpace(strings.TrimPrefix(week, "P")))
		return ok
	}
	return false
}

func parsePreseasonWeekNumber(week string) (int, bool) {
	week = strings.TrimSpace(strings.ToUpper(week))
	for _, prefix := range []string{"PRESEASON", "PRE", "PS", "P"} {
		if strings.HasPrefix(week, prefix) {
			week = strings.TrimSpace(strings.TrimPrefix(week, prefix))
			break
		}
	}
	n, err := strconv.Atoi(week)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func extractPreseasonWeeks(text string) []int {
	matches := preseasonWeekRe.FindAllStringSubmatch(text, -1)
	seen := map[int]bool{}
	var out []int
	for _, match := range matches {
		n, err := strconv.Atoi(match[1])
		if err != nil || n <= 0 || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

func extractSpanishWeeks(text string) []string {
	matches := spanishWeekRe.FindAllStringSubmatch(text, -1)
	seen := map[string]bool{}
	var out []string
	for _, match := range matches {
		if seen[match[1]] {
			continue
		}
		seen[match[1]] = true
		out = append(out, match[1])
	}
	return out
}

func extractGameNumberWeeks(text string) []string {
	matches := gameNumberRe.FindAllStringSubmatch(text, -1)
	seen := map[string]bool{}
	var out []string
	for _, match := range matches {
		if seen[match[1]] {
			continue
		}
		seen[match[1]] = true
		out = append(out, match[1])
	}
	return out
}

func ClassifyContentRole(title, description, channel string, durationSec int) ContentRole {
	text := NormalizeText(title + " " + description)
	switch {
	case containsNormalizedTerm(text, "radio broadcast") || containsNormalizedTerm(text, "radio edit") ||
		containsNormalizedTerm(text, "radio merged") || containsNormalizedTerm(text, "radio"):
		return ContentRoleRadio
	case containsNormalizedTerm(text, "game film") || containsNormalizedTerm(text, "team film"):
		return ContentRoleGameFilm
	case detectPartInfo(title+" "+description).Part > 0:
		return ContentRolePart
	case containsNormalizedTerm(text, "full game") || containsNormalizedTerm(text, "complete game") ||
		containsNormalizedTerm(text, "broadcast"):
		return ContentRoleFullGame
	case containsNormalizedTerm(text, "highlight") || containsNormalizedTerm(text, "highlights") ||
		containsNormalizedTerm(text, "resumen"):
		if isOfficialNFLChannel(channel) {
			return ContentRoleOfficialHighlight
		}
		return ContentRoleHighlight
	case containsNormalizedTerm(text, "story") || containsNormalizedTerm(text, "controversy") ||
		containsNormalizedTerm(text, "worst") || strings.Contains(text, "the time"):
		return ContentRoleStory
	case durationSec >= 5400:
		return ContentRoleFullGame
	default:
		return ContentRoleUnknown
	}
}

func DetectPartInfo(title, description string) PartInfo {
	return detectPartInfo(title + " " + description)
}

func detectPartInfo(text string) PartInfo {
	if match := partOfRe.FindStringSubmatch(text); len(match) > 0 {
		part, _ := strconv.Atoi(match[1])
		total := 0
		if len(match) > 2 && match[2] != "" {
			total, _ = strconv.Atoi(match[2])
		}
		return PartInfo{Part: part, Total: total}
	}
	if match := halfRe.FindStringSubmatch(text); len(match) > 0 {
		if strings.HasPrefix(strings.ToLower(match[1]), "1") || strings.EqualFold(match[1], "first") {
			return PartInfo{Part: 1, Total: 2}
		}
		return PartInfo{Part: 2, Total: 2}
	}
	if match := hashPartRe.FindStringSubmatch(text); len(match) > 0 {
		part, _ := strconv.Atoi(match[1])
		return PartInfo{Part: part}
	}
	return PartInfo{}
}

func MultipartGroupKey(title, description, channel string) string {
	part := DetectPartInfo(title, description)
	if part.Part == 0 {
		return ""
	}
	text := title + " " + description
	text = partStripRe.ReplaceAllString(text, " ")
	text = halfStripRe.ReplaceAllString(text, " ")
	text = hashPartStripRe.ReplaceAllString(text, " ")
	return NormalizeText(channel + " " + text)
}

func isOfficialNFLChannel(channel string) bool {
	norm := NormalizeText(channel)
	return norm == "nfl" || norm == "nfl throwback" || norm == "miami dolphins"
}
