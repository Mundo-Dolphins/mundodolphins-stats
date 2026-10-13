package hugodata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/assets"
	"github.com/Mundo-Dolphins/mundodolphins-stats/boxscore"
	"github.com/Mundo-Dolphins/mundodolphins-stats/pagejson"
	"github.com/Mundo-Dolphins/mundodolphins-stats/pfr"
	"github.com/Mundo-Dolphins/mundodolphins-stats/util"
)

// GenerateOptions controls optional behaviors during Hugo data generation.
type GenerateOptions struct {
	EnsureAssets bool
	// Assets downloads the remote files (team logos) when EnsureAssets is set.
	// The library provides no implementation; EnsureAssets without one is an error.
	Assets assets.Downloader
	// GamebookParser reads game book PDFs (optional; the library ships none).
	GamebookParser  pagejson.GamebookParser
	AssetsDir       string
	VideosCacheFile string
	ProcessedDir    string
}

// GenerateSeasonData writes season presentation data to JSON file
func GenerateSeasonData(ctx context.Context, env *pfr.Envelope[pfr.SeasonDataV1_1], outputDir string) error {
	return GenerateSeasonDataWithOptions(ctx, env, outputDir, GenerateOptions{})
}

// GenerateSeasonDataWithOptions writes season presentation data to JSON file
// and optionally ensures required assets are present locally.
func GenerateSeasonDataWithOptions(
	ctx context.Context,
	env *pfr.Envelope[pfr.SeasonDataV1_1],
	outputDir string,
	opts GenerateOptions,
) error {
	_ = ctx

	year := env.Season
	raw, err := toGenericMap(env)
	if err != nil {
		return fmt.Errorf("convert season envelope to generic map: %w", err)
	}
	builder := pagejson.NewSeasonBuilder(opts.ProcessedDir)
	if ts, ok := parseRFC3339(env.GeneratedAt); ok {
		builder.SetTime(ts)
	}
	doc, err := builder.Build(raw)
	if err != nil {
		return fmt.Errorf("build sectioned season page: %w", err)
	}

	// Create output directory
	dir := pagejson.HistoricalDataPath(outputDir, "seasons")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal to JSON
	jsonBytes, err := pagejson.EncodeToJSON(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal season data: %w", err)
	}

	// Write to file
	filename := filepath.Join(dir, fmt.Sprintf("%d.json", year))
	if err := os.WriteFile(filename, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write season data to %s: %w", filename, err)
	}

	if opts.EnsureAssets {
		assetsDir := strings.TrimSpace(opts.AssetsDir)
		if assetsDir == "" {
			assetsDir = filepath.Join(outputDir, "assets")
		}
		if err := ensureSeasonLogo(ctx, env, assetsDir, opts.Assets); err != nil {
			return fmt.Errorf("ensure season assets for %d: %w", year, err)
		}
	}

	return nil
}

// GenerateSeasonStatsData writes season stats presentation data to JSON file
func GenerateSeasonStatsData(ctx context.Context, env *pfr.Envelope[pfr.SeasonDataV1_1], outputDir string) error {
	_ = ctx

	year := env.Season
	raw, err := toGenericMap(env)
	if err != nil {
		return fmt.Errorf("convert season envelope to generic map: %w", err)
	}
	builder := pagejson.NewSeasonStatsBuilder("")
	if ts, ok := parseRFC3339(env.GeneratedAt); ok {
		builder.SetTime(ts)
	}
	doc, err := builder.Build(raw)
	if err != nil {
		return fmt.Errorf("build sectioned season-stats page: %w", err)
	}

	// Create output directory
	dir := pagejson.HistoricalDataPath(outputDir, "season_stats")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal to JSON
	jsonBytes, err := pagejson.EncodeToJSON(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal season stats data: %w", err)
	}

	// Write to file
	filename := filepath.Join(dir, fmt.Sprintf("%d.json", year))
	if err := os.WriteFile(filename, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write season stats data to %s: %w", filename, err)
	}

	return nil
}

// ErrNoAssetsDownloader is returned when assets are requested without a downloader.
var ErrNoAssetsDownloader = errors.New("hugodata: EnsureAssets needs GenerateOptions.Assets (the library ships no downloader)")

func ensureSeasonLogo(ctx context.Context, env *pfr.Envelope[pfr.SeasonDataV1_1], assetsDir string, dl assets.Downloader) error {
	if env == nil || strings.TrimSpace(assetsDir) == "" {
		return nil
	}
	main, ok := env.Data["main"]
	if !ok || main.Logo == nil || strings.TrimSpace(main.Logo.Src) == "" {
		return nil
	}

	logoURL := strings.TrimSpace(main.Logo.Src)
	parsed, err := url.Parse(logoURL)
	if err != nil || strings.TrimSpace(parsed.Path) == "" {
		return nil
	}

	filename := filepath.Base(parsed.Path)
	if filename == "" || filename == "." || filename == "/" {
		return nil
	}

	if dl == nil {
		return ErrNoAssetsDownloader
	}
	logoDir := filepath.Join(assetsDir, "logos")
	return dl.EnsureDownloaded(ctx, logoURL, filepath.Join(logoDir, filename))
}

// GenerateGameData writes game presentation data to JSON file
func GenerateGameData(ctx context.Context, env *boxscore.BoxscoreEnvelope, outputDir string) error {
	return GenerateGameDataWithOptions(ctx, env, outputDir, GenerateOptions{})
}

// GenerateGameDataWithOptions writes game presentation data to JSON file
// and optionally publishes locally cached gamebook assets.
func GenerateGameDataWithOptions(ctx context.Context, env *boxscore.BoxscoreEnvelope, outputDir string, opts GenerateOptions) error {
	_ = ctx

	if env == nil {
		return fmt.Errorf("boxscore envelope is nil")
	}

	gameID := env.GameID

	season, err := util.NFLSeasonFromGameID(gameID)
	if err != nil {
		return fmt.Errorf("failed to extract season from game ID %s: %w", gameID, err)
	}

	if opts.EnsureAssets {
		if err := ensureGamebookAsset(env, outputDir, opts, season); err != nil {
			return fmt.Errorf("ensure gamebook asset for %s: %w", gameID, err)
		}
	}

	builder := pagejson.NewGameBuilder(opts.ProcessedDir, opts.VideosCacheFile).SetGamebookParser(opts.GamebookParser)
	if ts, ok := parseRFC3339(env.GeneratedAt); ok {
		builder.SetTime(ts)
	}
	doc, err := builder.BuildEnvelope(*env)
	if err != nil {
		return fmt.Errorf("build sectioned game page: %w", err)
	}

	dir := pagejson.HistoricalDataPath(outputDir, "games", fmt.Sprintf("%d", season))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal to JSON
	jsonBytes, err := pagejson.EncodeToJSON(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal game data: %w", err)
	}

	// Write to file
	filename := filepath.Join(dir, fmt.Sprintf("%s.json", gameID))
	if err := os.WriteFile(filename, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write game data to %s: %w", filename, err)
	}

	return nil
}

func ensureGamebookAsset(env *boxscore.BoxscoreEnvelope, outputDir string, opts GenerateOptions, season int) error {
	if env == nil || env.Data.NFLCom == nil || env.Data.NFLCom.Gamebook == nil {
		return nil
	}

	gamebook := env.Data.NFLCom.Gamebook
	src := strings.TrimSpace(gamebook.Path)
	if src == "" {
		return nil
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil
	}
	if srcInfo.IsDir() {
		return nil
	}

	assetsDir := strings.TrimSpace(opts.AssetsDir)
	if assetsDir == "" {
		assetsDir = filepath.Join(outputDir, "assets")
	}

	ext := filepath.Ext(strings.TrimSpace(gamebook.Filename))
	if ext == "" {
		ext = filepath.Ext(src)
	}
	if ext == "" {
		ext = ".pdf"
	}
	filename := filepath.Base(env.GameID + ext)

	relPath := filepath.ToSlash(filepath.Join("assets", "gamebooks", fmt.Sprintf("%d", season), filename))
	dst := filepath.Join(assetsDir, "gamebooks", fmt.Sprintf("%d", season), filename)
	if err := copyFileIfChanged(src, dst, srcInfo); err != nil {
		return err
	}
	gamebook.Path = relPath
	return nil
}

func copyFileIfChanged(src, dst string, srcInfo os.FileInfo) error {
	if dstInfo, err := os.Stat(dst); err == nil && dstInfo.Size() == srcInfo.Size() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create asset directory %s: %w", filepath.Dir(dst), err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source asset %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create asset %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy asset %s to %s: %w", src, dst, err)
	}
	return nil
}

func toGenericMap[T any](v T) (map[string]interface{}, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func parseRFC3339(v string) (time.Time, bool) {
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// LoadSeasonFile loads and parses a season JSON file
func LoadSeasonFile(filename string) (*pfr.Envelope[pfr.SeasonDataV1_1], error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read season file %s: %w", filename, err)
	}

	var env pfr.Envelope[pfr.SeasonDataV1_1]
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("failed to parse season file %s: %w", filename, err)
	}

	return &env, nil
}

// LoadGameFile loads and parses a game JSON file
func LoadGameFile(filename string) (*boxscore.BoxscoreEnvelope, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read game file %s: %w", filename, err)
	}

	var env boxscore.BoxscoreEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("failed to parse game file %s: %w", filename, err)
	}

	return &env, nil
}
