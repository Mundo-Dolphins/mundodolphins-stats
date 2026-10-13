package hugodata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/boxscore"
	"github.com/Mundo-Dolphins/mundodolphins-stats/nflcom"
	"github.com/Mundo-Dolphins/mundodolphins-stats/pfr"
	"github.com/Mundo-Dolphins/mundodolphins-stats/videos"
)

// TestGenerateSeasonDataDirectoryCreation tests that directories are created
func TestGenerateSeasonDataDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()

	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Team:   pfr.TeamInfo{Name: "Miami Dolphins"},
		Data:   pfr.SeasonDataV1_1{},
	}

	err := GenerateSeasonData(context.Background(), env, tmpDir)
	if err != nil {
		t.Fatalf("GenerateSeasonData failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "data", "historical", "seasons", "2024.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("File not created: %s", expectedFile)
	}
}

// fakeDownloader stands in for the downloader that callers provide: it writes a
// placeholder file instead of reaching the network.
type fakeDownloader struct{ calls []string }

func (f *fakeDownloader) EnsureDownloaded(_ context.Context, url, dst string) error {
	f.calls = append(f.calls, url)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, []byte("PNGDATA"), 0o644)
}

func TestEnsureAssetsWithoutDownloaderIsAnError(t *testing.T) {
	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Data: pfr.SeasonDataV1_1{
			"main": pfr.PagePayload{Logo: &pfr.LogoInfo{Src: "https://example.com/logo.png"}},
		},
	}
	err := GenerateSeasonDataWithOptions(context.Background(), env, t.TempDir(), GenerateOptions{EnsureAssets: true})
	if !errors.Is(err, ErrNoAssetsDownloader) {
		t.Fatalf("expected ErrNoAssetsDownloader, got %v", err)
	}
}

func TestGenerateSeasonDataWithOptions_EnsureAssetsDownloadsLogo(t *testing.T) {
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	defer server.Close()

	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Data: pfr.SeasonDataV1_1{
			"main": pfr.PagePayload{
				Logo: &pfr.LogoInfo{Src: server.URL + "/mia-2024.png"},
			},
		},
	}

	err := GenerateSeasonDataWithOptions(context.Background(), env, tmpDir, GenerateOptions{EnsureAssets: true, Assets: &fakeDownloader{}})
	if err != nil {
		t.Fatalf("GenerateSeasonDataWithOptions failed: %v", err)
	}

	assetFile := filepath.Join(tmpDir, "assets", "logos", "mia-2024.png")
	if _, err := os.Stat(assetFile); err != nil {
		t.Fatalf("expected downloaded logo asset, got error: %v", err)
	}
}

// TestGenerateSeasonStatsDataDirectoryCreation tests that directories are created
func TestGenerateSeasonStatsDataDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()

	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Data: pfr.SeasonDataV1_1{
			"main": pfr.PagePayload{},
		},
	}

	err := GenerateSeasonStatsData(context.Background(), env, tmpDir)
	if err != nil {
		t.Fatalf("GenerateSeasonStatsData failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "data", "historical", "season_stats", "2024.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("File not created: %s", expectedFile)
	}
}

// TestGenerateGameDataDirectoryCreation tests that game directories are created
func TestGenerateGameDataDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()

	env := &boxscore.BoxscoreEnvelope{
		GameID: "202412210mia",
		Data: boxscore.BoxscoreData{
			YouTube: videos.YouTubeBlock{Status: "unknown"},
			Tables:  make(map[string]boxscore.TableResult),
		},
	}

	err := GenerateGameData(context.Background(), env, tmpDir)
	if err != nil {
		t.Fatalf("GenerateGameData failed: %v", err)
	}

	// Game ID 202412210mia -> season 2024
	expectedFile := filepath.Join(tmpDir, "data", "historical", "games", "2024", "202412210mia.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("File not created: %s", expectedFile)
	}
}

func TestGenerateGameDataWithOptionsPublishesGamebookAsset(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	sourceGamebook := filepath.Join(cacheDir, "nflcom", "gamebooks", "2024", "dolphins-at-browns-2024-reg-17", "gamebook.pdf")
	if err := os.MkdirAll(filepath.Dir(sourceGamebook), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(sourceGamebook, []byte("%PDF-1.7 test"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	env := &boxscore.BoxscoreEnvelope{
		GameID: "202412290cle",
		Data: boxscore.BoxscoreData{
			YouTube: videos.YouTubeBlock{Status: "unknown"},
			Tables:  make(map[string]boxscore.TableResult),
			NFLCom: &nflcom.GameData{
				Gamebook: &nflcom.GamebookLink{
					URL:      "https://static.www.nfl.com/gamecenter/gamebook.pdf",
					Filename: "gamebook.pdf",
					Path:     sourceGamebook,
				},
			},
		},
	}

	if err := GenerateGameDataWithOptions(context.Background(), env, tmpDir, GenerateOptions{EnsureAssets: true}); err != nil {
		t.Fatalf("GenerateGameDataWithOptions failed: %v", err)
	}

	assetPath := filepath.Join(tmpDir, "assets", "gamebooks", "2024", "202412290cle.pdf")
	if _, err := os.Stat(assetPath); err != nil {
		t.Fatalf("expected gamebook asset at %s: %v", assetPath, err)
	}

	generatedPath := filepath.Join(tmpDir, "data", "historical", "games", "2024", "202412290cle.json")
	generated, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("ReadFile generated game JSON failed: %v", err)
	}
	if !strings.Contains(string(generated), `"Gamebook oficial": "assets/gamebooks/2024/202412290cle.pdf"`) {
		t.Fatalf("generated JSON does not expose local gamebook asset path:\n%s", generated)
	}
	if strings.Contains(string(generated), sourceGamebook) {
		t.Fatalf("generated JSON leaked cache path %q", sourceGamebook)
	}
}

// TestLoadSeasonFileNotExist tests error handling for missing file
func TestLoadSeasonFileNotExist(t *testing.T) {
	_, err := LoadSeasonFile("/nonexistent/path/2024.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

// TestLoadGameFileNotExist tests error handling for missing file
func TestLoadGameFileNotExist(t *testing.T) {
	_, err := LoadGameFile("/nonexistent/path/game.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

// TestLoadSeasonFileValid tests successful season file loading
func TestLoadSeasonFileValid(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid season file
	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Team:   pfr.TeamInfo{Name: "Miami Dolphins", ID: "mia"},
		Data: pfr.SeasonDataV1_1{
			"main": pfr.PagePayload{
				Info: map[string]string{"key": "value"},
			},
		},
	}

	jsonData, _ := json.MarshalIndent(env, "", "  ")
	testFilepath := filepath.Join(tmpDir, "test_2024.json")
	os.WriteFile(testFilepath, jsonData, 0644)

	loaded, err := LoadSeasonFile(testFilepath)
	if err != nil {
		t.Fatalf("LoadSeasonFile failed: %v", err)
	}

	if loaded.Season != 2024 {
		t.Errorf("Season mismatch: got %d, want 2024", loaded.Season)
	}
}

// TestLoadGameFileValid tests successful game file loading
func TestLoadGameFileValid(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid game file
	env := &boxscore.BoxscoreEnvelope{
		GameID: "202412210mia",
		Data: boxscore.BoxscoreData{
			YouTube: videos.YouTubeBlock{Status: "unknown"},
			Tables:  make(map[string]boxscore.TableResult),
		},
	}

	jsonData, _ := json.MarshalIndent(env, "", "  ")
	testFilepath := filepath.Join(tmpDir, "test_game.json")
	os.WriteFile(testFilepath, jsonData, 0644)

	loaded, err := LoadGameFile(testFilepath)
	if err != nil {
		t.Fatalf("LoadGameFile failed: %v", err)
	}

	if loaded.GameID != "202412210mia" {
		t.Errorf("GameID mismatch: got %s, want 202412210mia", loaded.GameID)
	}
}

func TestParseRFC3339(t *testing.T) {
	if _, ok := parseRFC3339(""); ok {
		t.Fatalf("expected false for empty input")
	}

	if _, ok := parseRFC3339("not-a-time"); ok {
		t.Fatalf("expected false for invalid input")
	}

	v := "2026-03-20T10:00:00Z"
	got, ok := parseRFC3339(v)
	if !ok {
		t.Fatalf("expected ok for valid RFC3339 input")
	}
	want, _ := time.Parse(time.RFC3339, v)
	if !got.Equal(want) {
		t.Fatalf("unexpected parsed time: got=%v want=%v", got, want)
	}
}

func TestLoadSeasonFileInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := LoadSeasonFile(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadGameFileInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid_game.json")
	if err := os.WriteFile(path, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := LoadGameFile(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestEnsureSeasonLogo_NilEnv(t *testing.T) {
	err := ensureSeasonLogo(context.Background(), nil, "/tmp/assets", &fakeDownloader{})
	if err != nil {
		t.Errorf("expected nil error for nil env, got: %v", err)
	}
}

func TestGenerateSeasonDataWithOptions_WithGeneratedAt(t *testing.T) {
	tmpDir := t.TempDir()

	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season:      2025,
		GeneratedAt: "2026-03-20T10:00:00Z",
		Team:        pfr.TeamInfo{Name: "Miami Dolphins"},
		Data:        pfr.SeasonDataV1_1{},
	}

	err := GenerateSeasonDataWithOptions(context.Background(), env, tmpDir, GenerateOptions{})
	if err != nil {
		t.Fatalf("GenerateSeasonDataWithOptions failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "data", "historical", "seasons", "2025.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("File not created: %s", expectedFile)
	}
}

func TestGenerateSeasonStatsData_WithGeneratedAt(t *testing.T) {
	tmpDir := t.TempDir()

	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season:      2025,
		GeneratedAt: "2026-03-20T10:00:00Z",
		Data: pfr.SeasonDataV1_1{
			"main": pfr.PagePayload{},
		},
	}

	err := GenerateSeasonStatsData(context.Background(), env, tmpDir)
	if err != nil {
		t.Fatalf("GenerateSeasonStatsData failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "data", "historical", "season_stats", "2025.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("File not created: %s", expectedFile)
	}
}

func TestGenerateGameData_NilEnv(t *testing.T) {
	err := GenerateGameData(context.Background(), nil, "/tmp")
	if err == nil {
		t.Error("expected error for nil env")
	}
}

func TestGenerateGameData_WithGeneratedAt(t *testing.T) {
	tmpDir := t.TempDir()

	env := &boxscore.BoxscoreEnvelope{
		GameID:      "202412210mia",
		GeneratedAt: "2026-03-20T10:00:00Z",
		Data: boxscore.BoxscoreData{
			YouTube: videos.YouTubeBlock{Status: "unknown"},
			Tables:  make(map[string]boxscore.TableResult),
		},
	}

	err := GenerateGameData(context.Background(), env, tmpDir)
	if err != nil {
		t.Fatalf("GenerateGameData failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "data", "historical", "games", "2024", "202412210mia.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("File not created: %s", expectedFile)
	}
}

func TestGenerateGameData_UsesNFLSeasonForJanuaryGames(t *testing.T) {
	tmpDir := t.TempDir()

	env := &boxscore.BoxscoreEnvelope{
		GameID: "202601100mia",
		Data: boxscore.BoxscoreData{
			YouTube: videos.YouTubeBlock{Status: "unknown"},
			Tables:  map[string]boxscore.TableResult{},
		},
	}

	if err := GenerateGameData(context.Background(), env, tmpDir); err != nil {
		t.Fatalf("GenerateGameData failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "data", "historical", "games", "2025", "202601100mia.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Fatalf("expected January game under 2025 NFL season: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "data", "historical", "games", "2026", "202601100mia.json")); !os.IsNotExist(err) {
		t.Fatalf("January game must not be written under calendar year 2026")
	}
}

func TestGenerateGameData_InvalidGameIDReturnsError(t *testing.T) {
	err := GenerateGameData(context.Background(), &boxscore.BoxscoreEnvelope{
		GameID: "bad",
		Data: boxscore.BoxscoreData{
			YouTube: videos.YouTubeBlock{Status: "unknown"},
			Tables:  map[string]boxscore.TableResult{},
		},
	}, t.TempDir())
	if err == nil {
		t.Fatal("expected invalid game ID error")
	}
}

func TestGenerateSeasonData_OutputDirFileReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output")
	if err := os.WriteFile(outputFile, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2025,
		Team:   pfr.TeamInfo{Name: "Miami Dolphins"},
		Data:   pfr.SeasonDataV1_1{},
	}

	if err := GenerateSeasonData(context.Background(), env, outputFile); err == nil {
		t.Fatal("expected error when output directory is a file")
	}
}

func TestGenerateSeasonStatsData_OutputDirFileReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output")
	if err := os.WriteFile(outputFile, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2025,
		Data: pfr.SeasonDataV1_1{
			"main": pfr.PagePayload{},
		},
	}

	if err := GenerateSeasonStatsData(context.Background(), env, outputFile); err == nil {
		t.Fatal("expected error when output directory is a file")
	}
}

func TestGenerateSeasonDataWithOptions_EnsureAssets_NoLogo(t *testing.T) {
	tmpDir := t.TempDir()

	// Envelope without logo data → ensureSeasonLogo should no-op
	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Team:   pfr.TeamInfo{Name: "Miami Dolphins"},
		Data:   pfr.SeasonDataV1_1{},
	}

	err := GenerateSeasonDataWithOptions(context.Background(), env, tmpDir, GenerateOptions{EnsureAssets: true})
	if err != nil {
		t.Fatalf("GenerateSeasonDataWithOptions failed: %v", err)
	}
}

func TestGenerateSeasonData_EmptyData(t *testing.T) {
	tmpDir := t.TempDir()

	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Team:   pfr.TeamInfo{Name: "Miami Dolphins"},
		Data:   pfr.SeasonDataV1_1{},
	}

	err := GenerateSeasonData(context.Background(), env, tmpDir)
	if err != nil {
		t.Fatalf("GenerateSeasonData failed: %v", err)
	}
}

func TestEnsureSeasonLogo_EmptyAssetsDir(t *testing.T) {
	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Data:   pfr.SeasonDataV1_1{},
	}
	err := ensureSeasonLogo(context.Background(), env, "", &fakeDownloader{})
	if err != nil {
		t.Errorf("expected nil error for empty assetsDir, got: %v", err)
	}
}

func TestEnsureSeasonLogo_NoMainData(t *testing.T) {
	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Data:   pfr.SeasonDataV1_1{},
	}
	err := ensureSeasonLogo(context.Background(), env, "/tmp/assets", &fakeDownloader{})
	if err != nil {
		t.Errorf("expected nil error when no main data, got: %v", err)
	}
}

func TestEnsureSeasonLogo_NoLogoSrc(t *testing.T) {
	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Data: pfr.SeasonDataV1_1{
			"main": pfr.PagePayload{
				Logo: &pfr.LogoInfo{Src: ""},
			},
		},
	}
	err := ensureSeasonLogo(context.Background(), env, "/tmp/assets", &fakeDownloader{})
	if err != nil {
		t.Errorf("expected nil error when no logo src, got: %v", err)
	}
}

func TestEnsureSeasonLogo_InvalidURL(t *testing.T) {
	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Data: pfr.SeasonDataV1_1{
			"main": pfr.PagePayload{
				Logo: &pfr.LogoInfo{Src: "://invalid-url"},
			},
		},
	}
	err := ensureSeasonLogo(context.Background(), env, "/tmp/assets", &fakeDownloader{})
	if err != nil {
		t.Errorf("expected nil error for invalid URL, got: %v", err)
	}
}

func TestToGenericMap_MarshalFailure(t *testing.T) {
	// Use a type that can't be marshaled: a channel
	ch := make(chan int)
	_, err := toGenericMap(ch)
	if err == nil {
		t.Error("expected error for unmarshalable type")
	}
}

func TestGenerateSeasonDataWithOptions_ValidateArticle(t *testing.T) {
	tmpDir := t.TempDir()

	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season:      2024,
		GeneratedAt: "2026-03-20T10:00:00Z",
		Team:        pfr.TeamInfo{Name: "Miami Dolphins", ID: "mia"},
		Data: pfr.SeasonDataV1_1{
			"main": pfr.PagePayload{},
		},
	}

	err := GenerateSeasonDataWithOptions(context.Background(), env, tmpDir, GenerateOptions{
		EnsureAssets: true,
		AssetsDir:    filepath.Join(tmpDir, "custom-assets"),
	})
	if err != nil {
		t.Fatalf("GenerateSeasonDataWithOptions failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "data", "historical", "seasons", "2024.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("File not created: %s", expectedFile)
	}
}

func TestGenerateSeasonStatsData_WithNilMain(t *testing.T) {
	tmpDir := t.TempDir()

	env := &pfr.Envelope[pfr.SeasonDataV1_1]{
		Season: 2024,
		Data:   pfr.SeasonDataV1_1{},
	}

	err := GenerateSeasonStatsData(context.Background(), env, tmpDir)
	if err != nil {
		t.Fatalf("GenerateSeasonStatsData failed: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "data", "historical", "season_stats", "2024.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("File not created: %s", expectedFile)
	}
}
