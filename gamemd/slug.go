package gamemd

import (
	"fmt"
	"regexp"
	"strings"
)

// BuildSlug creates a deterministic public game slug from visitor/home team names and date.
func BuildSlug(visitor, home, dateISO string) string {
	visAbbr := getSlugAbbr(visitor)
	homeAbbr := getSlugAbbr(home)
	dateSlug := strings.ReplaceAll(dateISO, "-", "")
	return fmt.Sprintf("%s-%s-%s", visAbbr, homeAbbr, dateSlug)
}

func getSlugAbbr(name string) string {
	abbrs := map[string]string{
		"Arizona Cardinals":     "crd",
		"Atlanta Falcons":       "atl",
		"Baltimore Ravens":      "rav",
		"Buffalo Bills":         "buf",
		"Carolina Panthers":     "car",
		"Chicago Bears":         "chi",
		"Cincinnati Bengals":    "cin",
		"Cleveland Browns":      "cle",
		"Dallas Cowboys":        "dal",
		"Denver Broncos":        "den",
		"Detroit Lions":         "det",
		"Green Bay Packers":     "gnb",
		"Houston Oilers":        "oti",
		"Houston Texans":        "htx",
		"Indianapolis Colts":    "clt",
		"Jacksonville Jaguars":  "jax",
		"Kansas City Chiefs":    "kan",
		"Las Vegas Raiders":     "rai",
		"Los Angeles Chargers":  "sdg",
		"Los Angeles Rams":      "ram",
		"Miami Dolphins":        "mia",
		"Minnesota Vikings":     "min",
		"New England Patriots":  "nwe",
		"New Orleans Saints":    "nor",
		"New York Giants":       "nyg",
		"New York Jets":         "nyj",
		"Oakland Raiders":       "oak",
		"Philadelphia Eagles":   "phi",
		"Pittsburgh Steelers":   "pit",
		"San Diego Chargers":    "sdg",
		"San Francisco 49ers":   "sfo",
		"Seattle Seahawks":      "sea",
		"St. Louis Rams":        "ram",
		"Tampa Bay Buccaneers":  "tam",
		"Tennessee Titans":      "oti",
		"Washington Commanders": "was",
		"Washington Redskins":   "was",
	}
	if abbr, ok := abbrs[name]; ok {
		return abbr
	}
	clean := regexp.MustCompile(`[^a-zA-Z]`).ReplaceAllString(name, "")
	if len(clean) >= 3 {
		return strings.ToLower(clean[:3])
	}
	return "xxx"
}
