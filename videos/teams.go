// Package videos provides YouTube video discovery and caching for NFL games.
package videos

import "strings"

// TeamAliases holds the different name forms for an NFL team.
type TeamAliases struct {
	FullName string   // e.g. "Miami Dolphins"
	City     string   // e.g. "Miami"
	Mascot   string   // e.g. "Dolphins"
	Abbr     string   // e.g. "MIA"
	Extra    []string // additional aliases (e.g. historical names)
}

// AllTokens returns all searchable tokens for the team (lowercased).
func (t TeamAliases) AllTokens() []string {
	tokens := make([]string, 0, 5+len(t.Extra))
	if t.FullName != "" {
		tokens = append(tokens, strings.ToLower(t.FullName))
	}
	if t.City != "" {
		tokens = append(tokens, strings.ToLower(t.City))
	}
	if t.Mascot != "" {
		tokens = append(tokens, strings.ToLower(t.Mascot))
	}
	if t.Abbr != "" {
		tokens = append(tokens, strings.ToLower(t.Abbr))
	}
	for _, e := range t.Extra {
		if e != "" {
			tokens = append(tokens, strings.ToLower(e))
		}
	}
	return tokens
}

// nflTeams is the canonical registry of NFL team aliases.
// Includes historical names for teams that have relocated or rebranded.
var nflTeams = map[string]TeamAliases{
	"Miami Dolphins":           {FullName: "Miami Dolphins", City: "Miami", Mascot: "Dolphins", Abbr: "MIA"},
	"Buffalo Bills":            {FullName: "Buffalo Bills", City: "Buffalo", Mascot: "Bills", Abbr: "BUF"},
	"New England Patriots":     {FullName: "New England Patriots", City: "New England", Mascot: "Patriots", Abbr: "NWE", Extra: []string{"Boston Patriots"}},
	"Boston Patriots":          {FullName: "Boston Patriots", City: "Boston", Mascot: "Patriots", Abbr: "BOS", Extra: []string{"New England Patriots"}},
	"New York Jets":            {FullName: "New York Jets", City: "New York", Mascot: "Jets", Abbr: "NYJ"},
	"Baltimore Ravens":         {FullName: "Baltimore Ravens", City: "Baltimore", Mascot: "Ravens", Abbr: "RAV"},
	"Cincinnati Bengals":       {FullName: "Cincinnati Bengals", City: "Cincinnati", Mascot: "Bengals", Abbr: "CIN"},
	"Cleveland Browns":         {FullName: "Cleveland Browns", City: "Cleveland", Mascot: "Browns", Abbr: "CLE"},
	"Pittsburgh Steelers":      {FullName: "Pittsburgh Steelers", City: "Pittsburgh", Mascot: "Steelers", Abbr: "PIT"},
	"Houston Texans":           {FullName: "Houston Texans", City: "Houston", Mascot: "Texans", Abbr: "HTX"},
	"Indianapolis Colts":       {FullName: "Indianapolis Colts", City: "Indianapolis", Mascot: "Colts", Abbr: "CLT", Extra: []string{"Baltimore Colts"}},
	"Baltimore Colts":          {FullName: "Baltimore Colts", City: "Baltimore", Mascot: "Colts", Abbr: "CLT", Extra: []string{"Indianapolis Colts"}},
	"Jacksonville Jaguars":     {FullName: "Jacksonville Jaguars", City: "Jacksonville", Mascot: "Jaguars", Abbr: "JAX"},
	"Tennessee Titans":         {FullName: "Tennessee Titans", City: "Tennessee", Mascot: "Titans", Abbr: "OTI", Extra: []string{"Houston Oilers", "Tennessee Oilers"}},
	"Houston Oilers":           {FullName: "Houston Oilers", City: "Houston", Mascot: "Oilers", Abbr: "OTI", Extra: []string{"Tennessee Titans"}},
	"Tennessee Oilers":         {FullName: "Tennessee Oilers", City: "Tennessee", Mascot: "Oilers", Abbr: "OTI", Extra: []string{"Tennessee Titans"}},
	"Denver Broncos":           {FullName: "Denver Broncos", City: "Denver", Mascot: "Broncos", Abbr: "DEN"},
	"Kansas City Chiefs":       {FullName: "Kansas City Chiefs", City: "Kansas City", Mascot: "Chiefs", Abbr: "KAN", Extra: []string{"KC", "Dallas Texans"}},
	"Dallas Texans":            {FullName: "Dallas Texans", City: "Dallas", Mascot: "Texans", Abbr: "KAN", Extra: []string{"KC", "Kansas City Chiefs"}},
	"Las Vegas Raiders":        {FullName: "Las Vegas Raiders", City: "Las Vegas", Mascot: "Raiders", Abbr: "RAI", Extra: []string{"RAID", "Oakland Raiders", "Los Angeles Raiders"}},
	"Oakland Raiders":          {FullName: "Oakland Raiders", City: "Oakland", Mascot: "Raiders", Abbr: "RAI", Extra: []string{"RAID", "Las Vegas Raiders", "Los Angeles Raiders"}},
	"Los Angeles Raiders":      {FullName: "Los Angeles Raiders", City: "Los Angeles", Mascot: "Raiders", Abbr: "RAI", Extra: []string{"RAID", "Oakland Raiders", "Las Vegas Raiders"}},
	"Los Angeles Chargers":     {FullName: "Los Angeles Chargers", City: "Los Angeles", Mascot: "Chargers", Abbr: "SDG", Extra: []string{"SD", "San Diego Chargers"}},
	"San Diego Chargers":       {FullName: "San Diego Chargers", City: "San Diego", Mascot: "Chargers", Abbr: "SDG", Extra: []string{"SD", "Los Angeles Chargers"}},
	"Dallas Cowboys":           {FullName: "Dallas Cowboys", City: "Dallas", Mascot: "Cowboys", Abbr: "DAL"},
	"New York Giants":          {FullName: "New York Giants", City: "New York", Mascot: "Giants", Abbr: "NYG"},
	"Philadelphia Eagles":      {FullName: "Philadelphia Eagles", City: "Philadelphia", Mascot: "Eagles", Abbr: "PHI", Extra: []string{"PHIL"}},
	"Washington Commanders":    {FullName: "Washington Commanders", City: "Washington", Mascot: "Commanders", Abbr: "WAS", Extra: []string{"Washington Redskins", "Washington Football Team"}},
	"Washington Redskins":      {FullName: "Washington Redskins", City: "Washington", Mascot: "Redskins", Abbr: "WAS", Extra: []string{"Washington Commanders"}},
	"Washington Football Team": {FullName: "Washington Football Team", City: "Washington", Mascot: "Football Team", Abbr: "WAS", Extra: []string{"Washington Commanders"}},
	"Chicago Bears":            {FullName: "Chicago Bears", City: "Chicago", Mascot: "Bears", Abbr: "CHI"},
	"Detroit Lions":            {FullName: "Detroit Lions", City: "Detroit", Mascot: "Lions", Abbr: "DET"},
	"Green Bay Packers":        {FullName: "Green Bay Packers", City: "Green Bay", Mascot: "Packers", Abbr: "GNB"},
	"Minnesota Vikings":        {FullName: "Minnesota Vikings", City: "Minnesota", Mascot: "Vikings", Abbr: "MIN"},
	"Atlanta Falcons":          {FullName: "Atlanta Falcons", City: "Atlanta", Mascot: "Falcons", Abbr: "ATL"},
	"Carolina Panthers":        {FullName: "Carolina Panthers", City: "Carolina", Mascot: "Panthers", Abbr: "CAR"},
	"New Orleans Saints":       {FullName: "New Orleans Saints", City: "New Orleans", Mascot: "Saints", Abbr: "NOR", Extra: []string{"NO", "NOLA"}},
	"Tampa Bay Buccaneers":     {FullName: "Tampa Bay Buccaneers", City: "Tampa Bay", Mascot: "Buccaneers", Abbr: "TAM", Extra: []string{"Tampa Bay Bucs", "Bucs"}},
	"Arizona Cardinals":        {FullName: "Arizona Cardinals", City: "Arizona", Mascot: "Cardinals", Abbr: "CRD", Extra: []string{"St. Louis Cardinals", "Phoenix Cardinals"}},
	"St. Louis Cardinals":      {FullName: "St. Louis Cardinals", City: "St. Louis", Mascot: "Cardinals", Abbr: "CRD", Extra: []string{"Arizona Cardinals"}},
	"Phoenix Cardinals":        {FullName: "Phoenix Cardinals", City: "Phoenix", Mascot: "Cardinals", Abbr: "CRD", Extra: []string{"Arizona Cardinals"}},
	"Los Angeles Rams":         {FullName: "Los Angeles Rams", City: "Los Angeles", Mascot: "Rams", Abbr: "RAM", Extra: []string{"St. Louis Rams"}},
	"St. Louis Rams":           {FullName: "St. Louis Rams", City: "St. Louis", Mascot: "Rams", Abbr: "RAM", Extra: []string{"Los Angeles Rams"}},
	"San Francisco 49ers":      {FullName: "San Francisco 49ers", City: "San Francisco", Mascot: "49ers", Abbr: "SFO"},
	"Seattle Seahawks":         {FullName: "Seattle Seahawks", City: "Seattle", Mascot: "Seahawks", Abbr: "SEA"},
}

// LookupTeam finds team aliases by full name. Returns zero value if not found.
func LookupTeam(fullName string) TeamAliases {
	if a, ok := nflTeams[fullName]; ok {
		return a
	}
	// Fallback: try case-insensitive match
	lower := strings.ToLower(strings.TrimSpace(fullName))
	for name, a := range nflTeams {
		if strings.ToLower(name) == lower {
			return a
		}
	}
	// If not found, synthesize from the full name
	return synthesizeAliases(fullName)
}

// synthesizeAliases creates aliases from a team name string when not in the registry.
func synthesizeAliases(name string) TeamAliases {
	name = strings.TrimSpace(name)
	if name == "" {
		return TeamAliases{}
	}
	parts := strings.Fields(name)
	a := TeamAliases{FullName: name}
	if len(parts) >= 2 {
		a.City = strings.Join(parts[:len(parts)-1], " ")
		a.Mascot = parts[len(parts)-1]
	} else {
		a.City = name
		a.Mascot = name
	}
	return a
}
