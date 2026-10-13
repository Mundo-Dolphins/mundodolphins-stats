package articles

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	AuditSchemaName    = "mundodolphins.articles.audit"
	AuditSchemaVersion = "1.0.0"

	StatusReal  = "real"
	StatusDraft = "draft"
)

type AuditOptions struct {
	FromYear    int
	ToYear      int
	ArticlesDir string
}

type AuditReport struct {
	Schema       SchemaInfo    `json:"schema"`
	Range        YearRange     `json:"range"`
	ArticlesDir  string        `json:"articles_dir"`
	Present      []ArticleInfo `json:"present"`
	YearsPresent []int         `json:"years_present"`
	YearsReal    []int         `json:"years_real"`
	YearsDraft   []int         `json:"years_draft"`
	YearsMissing []int         `json:"years_missing"`
}

type SchemaInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type YearRange struct {
	From int `json:"from"`
	To   int `json:"to"`
}

type ArticleInfo struct {
	Year   int    `json:"year"`
	Status string `json:"status"`
	Path   string `json:"path"`
	Title  string `json:"title,omitempty"`
}

type DraftOptions struct {
	Year         int
	FromYear     int
	ToYear       int
	ArticlesDir  string
	ProcessedDir string
	GamesDir     string
	Force        bool
}

type DraftResult struct {
	Year      int      `json:"year"`
	Status    string   `json:"status"`
	Path      string   `json:"path"`
	Source    string   `json:"source,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Overwrote bool     `json:"overwrote,omitempty"`
}

type draftData struct {
	Year          int
	SourcePath    string
	SourceMissing bool
	Record        string
	Coach         string
	Division      string
	DivisionRank  string
	Leaders       []leader
	Draftees      []draftee
	GamesSummary  []string
	Warnings      []string
}

type leader struct {
	Label string
	Name  string
	Value string
}

type draftee struct {
	Round    string
	Pick     string
	Name     string
	Position string
	College  string
}

func Audit(opts AuditOptions) (AuditReport, error) {
	opts = normalizeAuditOptions(opts)
	if err := validateRange(opts.FromYear, opts.ToYear); err != nil {
		return AuditReport{}, err
	}

	byYear := map[int]ArticleInfo{}
	if entries, err := os.ReadDir(opts.ArticlesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			year, ok := parseYearFilename(entry.Name(), ".md")
			if !ok || year < opts.FromYear || year > opts.ToYear {
				continue
			}
			path := filepath.Join(opts.ArticlesDir, entry.Name())
			status, title := classifyArticle(path)
			byYear[year] = ArticleInfo{
				Year:   year,
				Status: status,
				Path:   filepath.Clean(path),
				Title:  title,
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return AuditReport{}, fmt.Errorf("read articles dir %s: %w", opts.ArticlesDir, err)
	}

	report := AuditReport{
		Schema:       SchemaInfo{Name: AuditSchemaName, Version: AuditSchemaVersion},
		Range:        YearRange{From: opts.FromYear, To: opts.ToYear},
		ArticlesDir:  filepath.Clean(opts.ArticlesDir),
		Present:      make([]ArticleInfo, 0, len(byYear)),
		YearsPresent: []int{},
		YearsReal:    []int{},
		YearsDraft:   []int{},
		YearsMissing: []int{},
	}
	for year := opts.FromYear; year <= opts.ToYear; year++ {
		info, ok := byYear[year]
		if !ok {
			report.YearsMissing = append(report.YearsMissing, year)
			continue
		}
		report.Present = append(report.Present, info)
		report.YearsPresent = append(report.YearsPresent, year)
		if info.Status == StatusDraft {
			report.YearsDraft = append(report.YearsDraft, year)
		} else {
			report.YearsReal = append(report.YearsReal, year)
		}
	}
	return report, nil
}

func GenerateDrafts(opts DraftOptions) ([]DraftResult, error) {
	opts = normalizeDraftOptions(opts)
	if err := validateRange(opts.FromYear, opts.ToYear); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opts.ArticlesDir, 0755); err != nil {
		return nil, fmt.Errorf("create articles dir %s: %w", opts.ArticlesDir, err)
	}

	results := make([]DraftResult, 0, opts.ToYear-opts.FromYear+1)
	for year := opts.FromYear; year <= opts.ToYear; year++ {
		path := filepath.Join(opts.ArticlesDir, fmt.Sprintf("%d.md", year))
		if _, err := os.Stat(path); err == nil && !opts.Force {
			results = append(results, DraftResult{Year: year, Status: "skipped_existing", Path: filepath.Clean(path)})
			continue
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("stat article %s: %w", path, err)
		}

		data := buildDraftData(year, opts.ProcessedDir, opts.GamesDir)
		content := RenderDraft(data)
		_, existedErr := os.Stat(path)
		overwrote := existedErr == nil
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("write draft article %s: %w", path, err)
		}
		status := "created"
		if overwrote {
			status = "overwritten"
		}
		results = append(results, DraftResult{
			Year:      year,
			Status:    status,
			Path:      filepath.Clean(path),
			Source:    data.SourcePath,
			Warnings:  append([]string(nil), data.Warnings...),
			Overwrote: overwrote,
		})
	}
	return results, nil
}

func RenderDraft(data draftData) string {
	var b bytes.Buffer
	source := data.SourcePath
	if source == "" {
		source = "pendiente"
	}
	fmt.Fprintf(&b, "---\n")
	fmt.Fprintf(&b, "title: %q\n", fmt.Sprintf("Temporada %d Miami Dolphins", data.Year))
	fmt.Fprintf(&b, "season: %d\n", data.Year)
	fmt.Fprintf(&b, "article_status: draft\n")
	fmt.Fprintf(&b, "draft: true\n")
	fmt.Fprintf(&b, "needs_editorial_review: true\n")
	fmt.Fprintf(&b, "generated_from: %q\n", source)
	fmt.Fprintf(&b, "---\n\n")

	fmt.Fprintf(&b, "# Temporada %d Miami Dolphins\n\n", data.Year)
	fmt.Fprintf(&b, "> Borrador generado automáticamente a partir de datos estructurados locales. No publicar sin revisión editorial.\n\n")

	fmt.Fprintf(&b, "## Datos verificados\n\n")
	writeFact(&b, "Temporada", fmt.Sprintf("%d", data.Year))
	writeFact(&b, "Récord", data.Record)
	writeFact(&b, "Entrenador", data.Coach)
	writeDivisionFact(&b, data.Division, data.DivisionRank)
	if data.SourceMissing {
		writeFact(&b, "Datos estructurados de temporada", "pendiente: no se encontró JSON procesado local")
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "## Líderes estadísticos\n\n")
	if len(data.Leaders) == 0 {
		b.WriteString("- Pendiente: no hay tablas estadísticas locales suficientes para derivar líderes.\n")
	} else {
		for _, item := range data.Leaders {
			fmt.Fprintf(&b, "- **%s:** %s (%s)\n", item.Label, item.Name, item.Value)
		}
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "## Draft destacado\n\n")
	if len(data.Draftees) == 0 {
		b.WriteString("- Pendiente: no hay tabla de draft o `derived.draftees` local para esta temporada.\n")
	} else {
		for _, item := range data.Draftees {
			parts := []string{}
			if item.Round != "" {
				parts = append(parts, "Ronda "+item.Round)
			}
			if item.Pick != "" {
				parts = append(parts, "pick "+item.Pick)
			}
			if item.Position != "" {
				parts = append(parts, item.Position)
			}
			if item.College != "" {
				parts = append(parts, item.College)
			}
			detail := strings.Join(parts, ", ")
			if detail == "" {
				fmt.Fprintf(&b, "- %s\n", item.Name)
			} else {
				fmt.Fprintf(&b, "- %s (%s)\n", item.Name, detail)
			}
		}
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "## Partidos y playoffs\n\n")
	if len(data.GamesSummary) == 0 {
		b.WriteString("- Pendiente: no hay calendario o partidos procesados locales suficientes para resumir esta sección.\n")
	} else {
		for _, item := range data.GamesSummary {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "## Pendiente de revisión editorial\n\n")
	b.WriteString("- Añadir contexto narrativo solo después de contrastarlo con fuentes editoriales o datos procesados.\n")
	b.WriteString("- Revisar nombres propios, lesiones, transacciones y evolución de la temporada antes de publicar.\n")
	b.WriteString("- Cambiar `article_status: draft` y `draft: true` cuando el artículo esté revisado.\n")

	return b.String()
}

func writeFact(b *bytes.Buffer, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "pendiente"
	}
	fmt.Fprintf(b, "- **%s:** %s\n", label, value)
}

func writeDivisionFact(b *bytes.Buffer, division, rank string) {
	division = strings.TrimSpace(division)
	rank = strings.TrimSpace(rank)
	if division == "" && rank == "" {
		writeFact(b, "División", "")
		return
	}
	if division != "" && rank != "" {
		fmt.Fprintf(b, "- **División:** %s, posición %s\n", division, rank)
		return
	}
	if division != "" {
		writeFact(b, "División", division)
		return
	}
	writeFact(b, "Posición divisional", rank)
}

func buildDraftData(year int, processedDir, gamesDir string) draftData {
	data := draftData{Year: year}
	sourcePath := filepath.Join(processedDir, fmt.Sprintf("%d.json", year))
	data.SourcePath = filepath.Clean(sourcePath)

	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			data.SourceMissing = true
			data.Warnings = append(data.Warnings, "processed season JSON missing")
			data.GamesSummary = gameFileSummary(year, gamesDir)
			return data
		}
		data.SourceMissing = true
		data.Warnings = append(data.Warnings, fmt.Sprintf("failed to read processed season JSON: %v", err))
		data.GamesSummary = gameFileSummary(year, gamesDir)
		return data
	}

	var season map[string]any
	if err := json.Unmarshal(raw, &season); err != nil {
		data.SourceMissing = true
		data.Warnings = append(data.Warnings, fmt.Sprintf("failed to parse processed season JSON: %v", err))
		data.GamesSummary = gameFileSummary(year, gamesDir)
		return data
	}

	if seasonYear, ok := intValue(season["season"]); ok && seasonYear > 0 {
		data.Year = seasonYear
	}
	main := nestedMap(season, "data", "main")
	advanced := nestedMap(season, "data", "advanced")
	if len(main) == 0 && len(advanced) > 0 {
		main = advanced
	}
	info := mapValue(main["info"])
	recordRaw := infoString(info, "Record", "record")
	_, recordRank, recordDivision := parseRecord(recordRaw)
	data.Record = cleanRecord(recordRaw)
	data.Coach = infoString(info, "Coach", "coach", "Head Coach", "head_coach")
	if data.Coach == "" {
		data.Coach = staffRole(main, "Head Coach")
	}
	data.Division = stringValue(firstPresent(main, "division_name", "division"))
	data.DivisionRank = stringValue(firstPresent(main, "division_rank", "divisionRank"))
	if data.Division == "" {
		data.Division = recordDivision
	}
	if data.DivisionRank == "" {
		data.DivisionRank = recordRank
	}

	tables := tablesMap(season, main)
	data.Leaders = buildLeaders(tables)
	data.Draftees = buildDraftees(season, tables)
	data.GamesSummary = append(data.GamesSummary, scheduleSummary(tables)...)
	data.GamesSummary = append(data.GamesSummary, gameFileSummary(data.Year, gamesDir)...)
	return data
}

func normalizeAuditOptions(opts AuditOptions) AuditOptions {
	if opts.FromYear == 0 {
		opts.FromYear = 1966
	}
	if opts.ToYear == 0 {
		opts.ToYear = 2026
	}
	if opts.ArticlesDir == "" {
		opts.ArticlesDir = "articles"
	}
	return opts
}

func normalizeDraftOptions(opts DraftOptions) DraftOptions {
	if opts.Year > 0 {
		opts.FromYear = opts.Year
		opts.ToYear = opts.Year
	}
	if opts.FromYear == 0 {
		opts.FromYear = 1966
	}
	if opts.ToYear == 0 {
		opts.ToYear = opts.FromYear
	}
	if opts.ArticlesDir == "" {
		opts.ArticlesDir = "articles"
	}
	if opts.ProcessedDir == "" {
		opts.ProcessedDir = "processed"
	}
	if opts.GamesDir == "" {
		opts.GamesDir = filepath.Join(opts.ProcessedDir, "games")
	}
	return opts
}

func validateRange(from, to int) error {
	if from <= 0 || to <= 0 {
		return fmt.Errorf("year range must be positive: from=%d to=%d", from, to)
	}
	if from > to {
		return fmt.Errorf("invalid year range: from=%d is after to=%d", from, to)
	}
	return nil
}

func classifyArticle(path string) (string, string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return StatusReal, ""
	}
	text := string(raw)
	status := StatusReal
	if frontmatter := extractFrontmatter(text); frontmatter != "" {
		fields := parseSimpleFrontmatter(frontmatter)
		if isTruthy(fields["draft"]) || strings.EqualFold(fields["article_status"], StatusDraft) || strings.EqualFold(fields["status"], StatusDraft) {
			status = StatusDraft
		}
	}
	return status, extractTitle(text)
}

func extractFrontmatter(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n")
		}
	}
	return ""
}

func parseSimpleFrontmatter(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		out[key] = value
	}
	return out
}

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1", "draft", "borrador":
		return true
	default:
		return false
	}
}

func extractTitle(text string) string {
	lines := strings.Split(text, "\n")
	inFrontmatter := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i == 0 && trimmed == "---" {
			inFrontmatter = true
			continue
		}
		if inFrontmatter {
			if trimmed == "---" {
				inFrontmatter = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			return strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		}
	}
	return ""
}

func parseYearFilename(name, suffix string) (int, bool) {
	if !strings.HasSuffix(name, suffix) {
		return 0, false
	}
	year, err := strconv.Atoi(strings.TrimSuffix(name, suffix))
	if err != nil {
		return 0, false
	}
	return year, true
}

func tablesMap(season map[string]any, main map[string]any) map[string]any {
	if tables := mapValue(main["tables"]); len(tables) > 0 {
		return tables
	}
	if tables := nestedMap(season, "data", "tables"); len(tables) > 0 {
		return tables
	}
	return map[string]any{}
}

func buildLeaders(tables map[string]any) []leader {
	specs := []struct {
		table string
		label string
		stat  []string
	}{
		{"passing", "Pase", []string{"pass_yds", "yds", "pass_yds_off"}},
		{"rushing_and_receiving", "Carrera", []string{"rush_yds", "yds", "rush_yds_off"}},
		{"rushing_and_receiving", "Recepción", []string{"rec_yds", "rec_yds_off"}},
		{"scoring", "Anotación", []string{"points", "pts"}},
		{"defense", "Placajes", []string{"tackles_combined", "tackles_solo"}},
		{"defense", "Capturas", []string{"sacks"}},
		{"defense", "Intercepciones", []string{"def_int", "int"}},
	}
	out := []leader{}
	seen := map[string]bool{}
	for _, spec := range specs {
		rows := tableRows(tables, spec.table)
		row, statKey, ok := maxRow(rows, spec.stat...)
		if !ok {
			continue
		}
		name := playerName(row)
		value := stringValue(row[statKey])
		if name == "" || value == "" || value == "0" {
			continue
		}
		key := spec.label + "\x00" + name + "\x00" + value
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, leader{Label: spec.label, Name: name, Value: value})
	}
	return out
}

func buildDraftees(season map[string]any, tables map[string]any) []draftee {
	for _, path := range [][]string{
		{"data", "draft", "derived", "draftees"},
		{"data", "main", "derived", "draftees"},
	} {
		rows := sliceValue(nestedValue(season, path...))
		if len(rows) > 0 {
			return drafteesFromRows(rows)
		}
	}
	for _, table := range []string{"draft", "draft_picks"} {
		rows := tableRows(tables, table)
		if len(rows) > 0 {
			return drafteesFromRows(rows)
		}
	}
	return nil
}

func drafteesFromRows(rows []map[string]any) []draftee {
	out := make([]draftee, 0, len(rows))
	for _, row := range rows {
		name := stringValue(firstPresent(row, "player", "Player", "name_display", "player_name", "Name"))
		if name == "" {
			continue
		}
		out = append(out, draftee{
			Round:    stringValue(firstPresent(row, "round", "Round", "Rnd", "draft_round")),
			Pick:     stringValue(firstPresent(row, "pick_overall", "pick", "Pick", "draft_pick", "Overall", "Pk")),
			Name:     name,
			Position: stringValue(firstPresent(row, "position", "Position", "Pos", "pos")),
			College:  stringValue(firstPresent(row, "college", "College", "college_name")),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rok := strconv.Atoi(out[i].Round)
		rj, rjok := strconv.Atoi(out[j].Round)
		if rok == nil && rjok == nil && ri != rj {
			return ri < rj
		}
		pi, pok := strconv.Atoi(out[i].Pick)
		pj, pjok := strconv.Atoi(out[j].Pick)
		if pok == nil && pjok == nil && pi != pj {
			return pi < pj
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

func scheduleSummary(tables map[string]any) []string {
	rows := tableRows(tables, "games")
	if len(rows) == 0 {
		return nil
	}
	total := 0
	playoffs := 0
	for _, row := range rows {
		if isSeparatorRow(row) {
			continue
		}
		if hasAny(row, "game_date", "date_game", "Date", "week_num", "Week") {
			total++
		}
		if rowMentionsPlayoffs(row) {
			playoffs++
		}
	}
	if total == 0 {
		return nil
	}
	out := []string{fmt.Sprintf("Calendario local: %d partidos en la tabla de temporada.", total)}
	if playoffs > 0 {
		out = append(out, fmt.Sprintf("Playoffs detectados en calendario local: %d partidos.", playoffs))
	}
	return out
}

func gameFileSummary(year int, gamesDir string) []string {
	if gamesDir == "" {
		return nil
	}
	pattern := filepath.Join(gamesDir, fmt.Sprintf("%d", year), "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("Partidos procesados localmente: %d JSON en %s.", len(matches), filepath.Clean(filepath.Dir(pattern)))}
}

func tableRows(tables map[string]any, key string) []map[string]any {
	table := mapValue(tables[key])
	if len(table) > 0 {
		if rows := sliceValue(table["rows"]); len(rows) > 0 {
			return rows
		}
	}
	return sliceValue(tables[key])
}

func maxRow(rows []map[string]any, keys ...string) (map[string]any, string, bool) {
	var best map[string]any
	bestKey := ""
	bestValue := 0.0
	for _, row := range rows {
		for _, key := range keys {
			value, ok := floatValue(row[key])
			if !ok {
				continue
			}
			if best == nil || value > bestValue {
				best = row
				bestKey = key
				bestValue = value
			}
		}
	}
	return best, bestKey, best != nil
}

func playerName(row map[string]any) string {
	return stringValue(firstPresent(row, "name_display", "player", "player_name", "name", "Player", "Name"))
}

func cleanRecord(record string) string {
	record = strings.TrimSpace(record)
	if record == "" {
		return ""
	}
	if before, _, ok := strings.Cut(record, ","); ok {
		return strings.TrimSpace(before)
	}
	return record
}

func parseRecord(record string) (main, rank, division string) {
	record = strings.TrimSpace(record)
	main = record
	if before, after, ok := strings.Cut(record, ","); ok {
		main = strings.TrimSpace(before)
		rest := strings.TrimSpace(after)
		if beforeIn, afterIn, ok := strings.Cut(rest, " in "); ok {
			rank = strings.TrimSpace(beforeIn)
			if beforeParen, _, ok := strings.Cut(afterIn, "("); ok {
				division = strings.TrimSpace(beforeParen)
			} else {
				division = strings.TrimSpace(afterIn)
			}
		}
	}
	return main, rank, division
}

func staffRole(page map[string]any, role string) string {
	for _, row := range sliceValue(page["staff"]) {
		if strings.EqualFold(stringValue(row["role"]), role) {
			return stringValue(row["name"])
		}
	}
	return ""
}

func infoString(info map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(info[key]); value != "" {
			return value
		}
	}
	for actual, value := range info {
		for _, key := range keys {
			if strings.EqualFold(actual, key) {
				if text := stringValue(value); text != "" {
					return text
				}
			}
		}
	}
	return ""
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	return nil
}

func nestedMap(root map[string]any, keys ...string) map[string]any {
	return mapValue(nestedValue(root, keys...))
}

func nestedValue(root map[string]any, keys ...string) any {
	var current any = root
	for _, key := range keys {
		m := mapValue(current)
		if len(m) == 0 {
			return nil
		}
		current = m[key]
	}
	return current
}

func mapValue(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	default:
		return nil
	}
}

func sliceValue(value any) []map[string]any {
	switch v := value.(type) {
	case []map[string]any:
		return v
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m := mapValue(item); len(m) > 0 {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func stringValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case map[string]any:
		if content := stringValue(v["content"]); content != "" {
			return content
		}
		if text := stringValue(v["Content"]); text != "" {
			return text
		}
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func intValue(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		return i, err == nil
	default:
		return 0, false
	}
}

func floatValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		v = strings.ReplaceAll(strings.TrimSpace(v), ",", "")
		if v == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func isSeparatorRow(row map[string]any) bool {
	for _, value := range row {
		text := strings.ToLower(stringValue(value))
		if text == "playoffs" {
			return true
		}
	}
	return false
}

func rowMentionsPlayoffs(row map[string]any) bool {
	for _, value := range row {
		text := strings.ToLower(stringValue(value))
		if strings.Contains(text, "wild card") ||
			strings.Contains(text, "divisional") ||
			strings.Contains(text, "championship") ||
			strings.Contains(text, "super bowl") {
			return true
		}
	}
	return false
}

func hasAny(row map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := row[key]; ok {
			return true
		}
	}
	return false
}
