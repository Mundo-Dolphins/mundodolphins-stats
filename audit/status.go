package audit

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/util"
	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

const (
	StatusSchemaName    = "mundodolphins.audit.status"
	StatusSchemaVersion = "1.0.0"
)

// StatusOptions configures the global offline status audit.
type StatusOptions struct {
	ProcessedDir       string
	GamesDir           string
	ArticlesDir        string
	VideosCacheFile    string
	OutputDir          string
	IncludeGeneratedAt bool
	Now                func() time.Time
}

// StatusManifest is the deterministic JSON report emitted by audit status.
type StatusManifest struct {
	Schema      StatusSchema      `json:"schema"`
	GeneratedAt string            `json:"generated_at,omitempty"`
	Inputs      []InputStatus     `json:"inputs"`
	Seasons     SeasonsStatus     `json:"seasons"`
	Games       GamesStatus       `json:"games"`
	Articles    ArticlesStatus    `json:"articles"`
	Videos      VideosStatus      `json:"videos"`
	Output      OutputStatus      `json:"output"`
	Schemas     []SchemaUsage     `json:"schemas"`
	Warnings    []ManifestWarning `json:"warnings"`
}

type StatusSchema struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InputStatus struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Kind   string `json:"kind"`
}

type SeasonsStatus struct {
	DetectedRange YearRange `json:"detected_range"`
	Total         int       `json:"total"`
	YearsPresent  []int     `json:"years_present"`
	YearsMissing  []int     `json:"years_missing"`
}

type YearRange struct {
	From int `json:"from,omitempty"`
	To   int `json:"to,omitempty"`
}

type GamesStatus struct {
	Total             int             `json:"total"`
	BySeason          []YearCount     `json:"by_season"`
	ByDirectorySeason []YearCount     `json:"by_directory_season,omitempty"`
	Misplaced         []MisplacedGame `json:"misplaced"`
}

type YearCount struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

type MisplacedGame struct {
	GameID         string `json:"game_id"`
	Path           string `json:"path"`
	LocatedSeason  int    `json:"located_season"`
	ExpectedSeason int    `json:"expected_season"`
}

type ArticlesStatus struct {
	Total        int   `json:"total"`
	YearsPresent []int `json:"years_present"`
	YearsMissing []int `json:"years_missing"`
}

type VideosStatus struct {
	Total                   int           `json:"total"`
	TotalItems              int           `json:"total_items"`
	TotalVideos             int           `json:"total_videos"`
	Statuses                []StatusCount `json:"statuses"`
	Coverage                VideoCoverage `json:"coverage"`
	ExtraCacheGameIDs       []string      `json:"extra_cache_game_ids,omitempty"`
	SelectedWithoutVideoIDs []string      `json:"selected_without_video_ids,omitempty"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type VideoCoverage struct {
	GamesTotal             int      `json:"games_total"`
	GamesWithCacheItem     int      `json:"games_with_cache_item"`
	GamesWithSelectedVideo int      `json:"games_with_selected_video"`
	GamesMissingVideo      int      `json:"games_missing_video"`
	MissingGameIDs         []string `json:"missing_game_ids,omitempty"`
}

type OutputStatus struct {
	TotalJSON        int         `json:"total_json"`
	SeasonYears      []int       `json:"season_years"`
	SeasonStatsYears []int       `json:"season_stats_years"`
	GamesTotal       int         `json:"games_total"`
	GamesBySeason    []YearCount `json:"games_by_season"`
	PagesTotal       int         `json:"pages_total"`
}

type SchemaUsage struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Count   int      `json:"count"`
	Sources []string `json:"sources"`
}

type ManifestWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type statusBuilder struct {
	opts           StatusOptions
	schemaCounts   map[string]*schemaCounter
	warnings       []ManifestWarning
	gameIDs        map[string]bool
	gameIDList     []string
	referenceYears map[int]bool
}

type schemaCounter struct {
	name    string
	version string
	count   int
	sources map[string]bool
}

type envelopeMeta struct {
	Schema struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"schema"`
	Season int    `json:"season"`
	GameID string `json:"game_id"`
}

// BuildStatusManifest creates a global offline inventory of local processed, generated,
// editorial, and video-cache data.
func BuildStatusManifest(opts StatusOptions) (StatusManifest, error) {
	opts = normalizeStatusOptions(opts)
	b := &statusBuilder{
		opts:           opts,
		schemaCounts:   map[string]*schemaCounter{},
		gameIDs:        map[string]bool{},
		referenceYears: map[int]bool{},
	}

	manifest := StatusManifest{
		Schema: StatusSchema{Name: StatusSchemaName, Version: StatusSchemaVersion},
		Inputs: []InputStatus{
			b.inputStatus("processed", opts.ProcessedDir, "directory"),
			b.inputStatus("games", opts.GamesDir, "directory"),
			b.inputStatus("articles", opts.ArticlesDir, "directory"),
			b.inputStatus("videos_cache", opts.VideosCacheFile, "file"),
			b.inputStatus("output", opts.OutputDir, "directory"),
		},
	}
	if opts.IncludeGeneratedAt {
		manifest.GeneratedAt = opts.Now().UTC().Format(time.RFC3339)
	}

	manifest.Seasons = b.auditSeasons()
	manifest.Games = b.auditGames()
	manifest.Articles = b.auditArticles(sortedYears(b.referenceYears))
	manifest.Videos = b.auditVideos()
	manifest.Output = b.auditOutput()
	manifest.Schemas = b.schemaUsages()
	manifest.Warnings = b.sortedWarnings()

	return manifest, nil
}

func normalizeStatusOptions(opts StatusOptions) StatusOptions {
	if opts.ProcessedDir == "" {
		opts.ProcessedDir = "processed"
	}
	if opts.GamesDir == "" {
		opts.GamesDir = filepath.Join(opts.ProcessedDir, "games")
	}
	if opts.ArticlesDir == "" {
		opts.ArticlesDir = "articles"
	}
	if opts.VideosCacheFile == "" {
		opts.VideosCacheFile = filepath.Join("cache", "videos", "videos.json")
	}
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join("output", "data")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return opts
}

func (b *statusBuilder) inputStatus(name, path, kind string) InputStatus {
	return InputStatus{
		Name:   name,
		Path:   filepath.Clean(path),
		Exists: pathExists(path),
		Kind:   kind,
	}
}

func (b *statusBuilder) auditSeasons() SeasonsStatus {
	years := map[int]bool{}
	if !dirExists(b.opts.ProcessedDir) {
		b.warn("input_missing", fmt.Sprintf("processed directory does not exist: %s", b.opts.ProcessedDir))
		return SeasonsStatus{YearsPresent: []int{}, YearsMissing: []int{}}
	}

	entries, err := os.ReadDir(b.opts.ProcessedDir)
	if err != nil {
		b.warn("read_processed_failed", fmt.Sprintf("failed to read processed directory %s: %v", b.opts.ProcessedDir, err))
		return SeasonsStatus{YearsPresent: []int{}, YearsMissing: []int{}}
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		year, ok := parseYearFilename(entry.Name(), ".json")
		if !ok {
			continue
		}
		path := filepath.Join(b.opts.ProcessedDir, entry.Name())
		meta, ok := b.readMeta(path, "processed")
		if ok && meta.Season > 0 {
			year = meta.Season
		}
		years[year] = true
		b.referenceYears[year] = true
	}

	present := sortedYears(years)
	missing := missingYears(present)
	if len(present) == 0 {
		b.warn("no_processed_seasons", fmt.Sprintf("no season JSON files found in %s", b.opts.ProcessedDir))
	}
	if len(missing) > 0 {
		b.warn("missing_seasons", fmt.Sprintf("missing processed season JSON files for years: %s", formatYears(missing)))
	}
	return SeasonsStatus{
		DetectedRange: rangeForYears(present),
		Total:         len(present),
		YearsPresent:  present,
		YearsMissing:  missing,
	}
}

func (b *statusBuilder) auditGames() GamesStatus {
	bySeason := map[int]int{}
	byDirectorySeason := map[int]int{}
	var misplaced []MisplacedGame

	if !dirExists(b.opts.GamesDir) {
		b.warn("input_missing", fmt.Sprintf("games directory does not exist: %s", b.opts.GamesDir))
		return GamesStatus{BySeason: []YearCount{}, Misplaced: []MisplacedGame{}}
	}

	var gamePaths []string
	err := filepath.WalkDir(b.opts.GamesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			b.warn("walk_games_failed", fmt.Sprintf("failed to access %s: %v", path, err))
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		gamePaths = append(gamePaths, path)
		return nil
	})
	if err != nil {
		b.warn("walk_games_failed", fmt.Sprintf("failed to walk games directory %s: %v", b.opts.GamesDir, err))
	}
	sort.Strings(gamePaths)

	for _, path := range gamePaths {
		meta, ok := b.readMeta(path, "games")
		gameID := strings.TrimSpace(meta.GameID)
		if gameID == "" {
			gameID = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		expectedSeason, err := seasonFromGameID(gameID)
		if err != nil {
			b.warn("invalid_game_id", fmt.Sprintf("cannot determine NFL season for game %s at %s: %v", gameID, path, err))
			continue
		}
		_ = ok
		bySeason[expectedSeason]++
		b.referenceYears[expectedSeason] = true
		if !b.gameIDs[gameID] {
			b.gameIDs[gameID] = true
			b.gameIDList = append(b.gameIDList, gameID)
		}

		if locatedSeason, ok := seasonFromPathParent(path); ok {
			byDirectorySeason[locatedSeason]++
			if locatedSeason != expectedSeason {
				misplaced = append(misplaced, MisplacedGame{
					GameID:         gameID,
					Path:           filepath.Clean(path),
					LocatedSeason:  locatedSeason,
					ExpectedSeason: expectedSeason,
				})
			}
		}
	}

	sort.Slice(misplaced, func(i, j int) bool {
		if misplaced[i].ExpectedSeason != misplaced[j].ExpectedSeason {
			return misplaced[i].ExpectedSeason < misplaced[j].ExpectedSeason
		}
		return misplaced[i].GameID < misplaced[j].GameID
	})
	sort.Strings(b.gameIDList)

	if len(gamePaths) == 0 {
		b.warn("no_games", fmt.Sprintf("no game JSON files found in %s", b.opts.GamesDir))
	}
	if len(misplaced) > 0 {
		b.warn("misplaced_games", fmt.Sprintf("%d game JSON files are not under their NFL season directory", len(misplaced)))
	}

	return GamesStatus{
		Total:             len(gamePaths),
		BySeason:          yearCounts(bySeason),
		ByDirectorySeason: yearCounts(byDirectorySeason),
		Misplaced:         misplaced,
	}
}

func (b *statusBuilder) auditArticles(referenceYears []int) ArticlesStatus {
	years := map[int]bool{}
	if !dirExists(b.opts.ArticlesDir) {
		b.warn("input_missing", fmt.Sprintf("articles directory does not exist: %s", b.opts.ArticlesDir))
		if len(referenceYears) > 0 {
			b.warn("missing_articles", fmt.Sprintf("missing editorial articles for years: %s", formatYears(referenceYears)))
		}
		return ArticlesStatus{YearsPresent: []int{}, YearsMissing: append([]int(nil), referenceYears...)}
	}

	entries, err := os.ReadDir(b.opts.ArticlesDir)
	if err != nil {
		b.warn("read_articles_failed", fmt.Sprintf("failed to read articles directory %s: %v", b.opts.ArticlesDir, err))
		return ArticlesStatus{YearsPresent: []int{}, YearsMissing: []int{}}
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		year, ok := parseYearFilename(entry.Name(), ".md")
		if ok {
			years[year] = true
		}
	}

	present := sortedYears(years)
	missing := missingReferenceYears(referenceYears, years)
	if len(missing) > 0 {
		b.warn("missing_articles", fmt.Sprintf("missing editorial articles for years: %s", formatYears(missing)))
	}
	return ArticlesStatus{
		Total:        len(present),
		YearsPresent: present,
		YearsMissing: missing,
	}
}

func (b *statusBuilder) auditVideos() VideosStatus {
	status := VideosStatus{
		Statuses: []StatusCount{},
		Coverage: VideoCoverage{
			GamesTotal:     len(b.gameIDList),
			MissingGameIDs: []string{},
		},
	}
	if !fileExists(b.opts.VideosCacheFile) {
		b.warn("input_missing", fmt.Sprintf("videos cache file does not exist: %s", b.opts.VideosCacheFile))
		status.Coverage.GamesMissingVideo = len(b.gameIDList)
		status.Coverage.MissingGameIDs = append(status.Coverage.MissingGameIDs, b.gameIDList...)
		return status
	}

	cf, err := videos.NewFileCache(b.opts.VideosCacheFile).Load()
	if err != nil {
		b.warn("read_videos_failed", fmt.Sprintf("failed to read videos cache %s: %v", b.opts.VideosCacheFile, err))
		status.Coverage.GamesMissingVideo = len(b.gameIDList)
		status.Coverage.MissingGameIDs = append(status.Coverage.MissingGameIDs, b.gameIDList...)
		return status
	}
	b.addSchema(cf.Schema.Name, cf.Schema.Version, "videos_cache")

	statusCounts := map[string]int{}
	var extra []string
	var selectedWithoutVideos []string

	for gameID, item := range cf.Items {
		status.TotalItems++
		status.Total++
		if item == nil {
			statusCounts["unknown"]++
			continue
		}
		itemStatus := strings.TrimSpace(item.Status)
		if itemStatus == "" {
			itemStatus = "unknown"
		}
		statusCounts[itemStatus]++
		status.TotalVideos += len(item.Videos)
		if !b.gameIDs[gameID] {
			extra = append(extra, gameID)
		}
		if videos.IsPublishableStatus(itemStatus) && len(item.Videos) == 0 {
			selectedWithoutVideos = append(selectedWithoutVideos, gameID)
		}
	}

	for _, gameID := range b.gameIDList {
		item := cf.Items[gameID]
		if item != nil {
			status.Coverage.GamesWithCacheItem++
		}
		if item != nil && videos.IsPublishableStatus(item.Status) && len(item.Videos) > 0 {
			status.Coverage.GamesWithSelectedVideo++
			continue
		}
		status.Coverage.MissingGameIDs = append(status.Coverage.MissingGameIDs, gameID)
	}
	status.Coverage.GamesMissingVideo = len(status.Coverage.MissingGameIDs)

	sort.Strings(extra)
	sort.Strings(selectedWithoutVideos)
	status.ExtraCacheGameIDs = extra
	status.SelectedWithoutVideoIDs = selectedWithoutVideos
	status.Statuses = statusCountsList(statusCounts)

	if status.Coverage.GamesMissingVideo > 0 {
		b.warn("videos_missing_for_games", fmt.Sprintf("%d game JSON files do not have a selected video", status.Coverage.GamesMissingVideo))
	}
	if len(extra) > 0 {
		b.warn("videos_cache_extra_games", fmt.Sprintf("%d videos cache entries do not match a processed game JSON", len(extra)))
	}
	if len(selectedWithoutVideos) > 0 {
		b.warn("videos_selected_without_media", fmt.Sprintf("%d cache entries are selected but contain no videos", len(selectedWithoutVideos)))
	}

	return status
}

func (b *statusBuilder) auditOutput() OutputStatus {
	status := OutputStatus{
		SeasonYears:      []int{},
		SeasonStatsYears: []int{},
		GamesBySeason:    []YearCount{},
	}
	if !dirExists(b.opts.OutputDir) {
		b.warn("input_missing", fmt.Sprintf("output data directory does not exist: %s", b.opts.OutputDir))
		return status
	}

	seasonYears := map[int]bool{}
	seasonStatsYears := map[int]bool{}
	gamesBySeason := map[int]int{}

	err := filepath.WalkDir(b.opts.OutputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			b.warn("walk_output_failed", fmt.Sprintf("failed to access %s: %v", path, err))
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		status.TotalJSON++
		b.readMeta(path, "output")

		rel, relErr := filepath.Rel(b.opts.OutputDir, path)
		if relErr != nil {
			return nil
		}
		parts := splitPath(rel)
		if len(parts) == 2 && parts[0] == "seasons" {
			if year, ok := parseYearFilename(parts[1], ".json"); ok {
				seasonYears[year] = true
			}
		}
		if len(parts) == 2 && parts[0] == "season_stats" {
			if year, ok := parseYearFilename(parts[1], ".json"); ok {
				seasonStatsYears[year] = true
			}
		}
		if len(parts) == 3 && parts[0] == "games" {
			if season, err := strconv.Atoi(parts[1]); err == nil {
				status.GamesTotal++
				gamesBySeason[season]++
			}
		}
		if len(parts) > 0 && parts[0] == "pages" {
			status.PagesTotal++
		}
		return nil
	})
	if err != nil {
		b.warn("walk_output_failed", fmt.Sprintf("failed to walk output data directory %s: %v", b.opts.OutputDir, err))
	}

	status.SeasonYears = sortedYears(seasonYears)
	status.SeasonStatsYears = sortedYears(seasonStatsYears)
	status.GamesBySeason = yearCounts(gamesBySeason)

	return status
}

func (b *statusBuilder) readMeta(path, source string) (envelopeMeta, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		b.warn("read_json_failed", fmt.Sprintf("failed to read JSON file %s: %v", path, err))
		return envelopeMeta{}, false
	}
	var meta envelopeMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		b.warn("parse_json_failed", fmt.Sprintf("failed to parse JSON file %s: %v", path, err))
		return envelopeMeta{}, false
	}
	if meta.Schema.Name != "" || meta.Schema.Version != "" {
		b.addSchema(meta.Schema.Name, meta.Schema.Version, source)
	}
	return meta, true
}

func (b *statusBuilder) addSchema(name, version, source string) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if name == "" {
		name = "unknown"
	}
	if version == "" {
		version = "unknown"
	}
	key := name + "\x00" + version
	counter := b.schemaCounts[key]
	if counter == nil {
		counter = &schemaCounter{name: name, version: version, sources: map[string]bool{}}
		b.schemaCounts[key] = counter
	}
	counter.count++
	counter.sources[source] = true
}

func (b *statusBuilder) schemaUsages() []SchemaUsage {
	out := make([]SchemaUsage, 0, len(b.schemaCounts))
	versionsByName := map[string]map[string]bool{}
	for _, counter := range b.schemaCounts {
		out = append(out, SchemaUsage{
			Name:    counter.name,
			Version: counter.version,
			Count:   counter.count,
			Sources: sortedStrings(counter.sources),
		})
		if versionsByName[counter.name] == nil {
			versionsByName[counter.name] = map[string]bool{}
		}
		versionsByName[counter.name][counter.version] = true
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Version < out[j].Version
	})
	for name, versions := range versionsByName {
		if len(versions) > 1 {
			b.warn("mixed_schema_versions", fmt.Sprintf("schema %s has multiple versions: %s", name, strings.Join(sortedStrings(versions), ", ")))
		}
	}
	return out
}

func (b *statusBuilder) warn(code, message string) {
	b.warnings = append(b.warnings, ManifestWarning{Code: code, Message: message})
}

func (b *statusBuilder) sortedWarnings() []ManifestWarning {
	out := append([]ManifestWarning(nil), b.warnings...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Message < out[j].Message
	})
	return out
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func parseYearFilename(name, suffix string) (int, bool) {
	if !strings.HasSuffix(name, suffix) {
		return 0, false
	}
	stem := strings.TrimSuffix(name, suffix)
	if len(stem) != 4 {
		return 0, false
	}
	year, err := strconv.Atoi(stem)
	if err != nil {
		return 0, false
	}
	return year, true
}

func sortedYears(years map[int]bool) []int {
	out := make([]int, 0, len(years))
	for year := range years {
		out = append(out, year)
	}
	sort.Ints(out)
	return out
}

func missingYears(present []int) []int {
	if len(present) < 2 {
		return []int{}
	}
	out := []int{}
	for year := present[0]; year <= present[len(present)-1]; year++ {
		if !intInSorted(present, year) {
			out = append(out, year)
		}
	}
	return out
}

func missingReferenceYears(reference []int, present map[int]bool) []int {
	out := []int{}
	for _, year := range reference {
		if !present[year] {
			out = append(out, year)
		}
	}
	return out
}

func intInSorted(values []int, target int) bool {
	idx := sort.SearchInts(values, target)
	return idx < len(values) && values[idx] == target
}

func rangeForYears(years []int) YearRange {
	if len(years) == 0 {
		return YearRange{}
	}
	return YearRange{From: years[0], To: years[len(years)-1]}
}

func formatYears(years []int) string {
	parts := make([]string, len(years))
	for i, year := range years {
		parts[i] = strconv.Itoa(year)
	}
	return strings.Join(parts, ", ")
}

func yearCounts(counts map[int]int) []YearCount {
	years := make([]int, 0, len(counts))
	for year := range counts {
		years = append(years, year)
	}
	sort.Ints(years)
	out := make([]YearCount, 0, len(years))
	for _, year := range years {
		out = append(out, YearCount{Year: year, Count: counts[year]})
	}
	return out
}

func statusCountsList(counts map[string]int) []StatusCount {
	statuses := make([]string, 0, len(counts))
	for status := range counts {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	out := make([]StatusCount, 0, len(statuses))
	for _, status := range statuses {
		out = append(out, StatusCount{Status: status, Count: counts[status]})
	}
	return out
}

func sortedStrings(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func seasonFromGameID(gameID string) (int, error) {
	if len(gameID) < 8 {
		return 0, fmt.Errorf("game ID has fewer than 8 date characters")
	}
	for i := 0; i < 8; i++ {
		if gameID[i] < '0' || gameID[i] > '9' {
			return 0, fmt.Errorf("game ID date prefix is not numeric")
		}
	}
	return util.NFLSeasonFromGameID(gameID)
}

func seasonFromPathParent(path string) (int, bool) {
	parent := filepath.Base(filepath.Dir(path))
	if len(parent) != 4 {
		return 0, false
	}
	year, err := strconv.Atoi(parent)
	if err != nil {
		return 0, false
	}
	return year, true
}

func splitPath(path string) []string {
	if path == "." || path == "" {
		return nil
	}
	return strings.Split(filepath.ToSlash(path), "/")
}
