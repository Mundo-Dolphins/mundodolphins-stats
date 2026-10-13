package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Mundo-Dolphins/mundodolphins-stats/pagejson"
	"github.com/Mundo-Dolphins/mundodolphins-stats/util"
	"github.com/spf13/cobra"
)

// jsonGamesCmd generates structured JSON content for games
var jsonGamesCmd = &cobra.Command{
	Use:     "game",
	Aliases: []string{"games"},
	Short:   "Generate historical JSON content for games",
	Long: `Generate historical JSON content for game pages from processed game JSON files.
Output is written to <output-dir>/data/historical/games/<season>/<game_id>.json.

Examples:
  # Generate for all games in a season
  dolphins-stats json games --year 2025

  # Generate for a specific game
  dolphins-stats json games --game-id 202512280mia

  # Generate for a range of seasons
  dolphins-stats json games --from-year 2023 --to-year 2025`,
	RunE: runJSONGames,
}

var (
	jsonGamesYear         int64
	jsonGamesFromYear     int64
	jsonGamesToYear       int64
	jsonGamesGameID       string
	jsonGamesOutputDir    string
	jsonGamesProcessedDir string
	jsonGamesVideosCache  string
)

func init() {
	jsonCmd.AddCommand(jsonGamesCmd)

	jsonGamesCmd.Flags().Int64Var(&jsonGamesYear, "year", 0,
		"Generate for games in specific season (0 = use range)")
	jsonGamesCmd.Flags().Int64Var(&jsonGamesFromYear, "from-year", 2023,
		"Start season for range generation")
	jsonGamesCmd.Flags().Int64Var(&jsonGamesToYear, "to-year", 2025,
		"End season for range generation")
	jsonGamesCmd.Flags().StringVar(&jsonGamesGameID, "id", "",
		"Generate for specific game by ID (e.g., 202512280mia)")
	jsonGamesCmd.Flags().StringVar(&jsonGamesGameID, "game-id", "",
		"Generate for specific game by ID (e.g., 202512280mia)")
	jsonGamesCmd.Flags().StringVar(&jsonGamesOutputDir, "output-dir", "output",
		"Output base directory; data is written below data/historical")
	jsonGamesCmd.Flags().StringVar(&jsonGamesProcessedDir, "processed-dir", "processed",
		"Input directory with processed game JSON files")
	jsonGamesCmd.Flags().StringVar(&jsonGamesVideosCache, "videos-cache", "cache/videos/videos.json",
		"Path to videos cache JSON file")
}

func runJSONGames(cmd *cobra.Command, args []string) error {
	if jsonGamesGameID != "" {
		// Generate for single game
		return generateJSONGamesSingle(jsonGamesGameID)
	}

	if jsonGamesYear != 0 {
		// Generate for all games in a single season
		return generateJSONGamesSeason(int(jsonGamesYear))
	}

	// Generate for range of seasons
	return generateJSONGamesRange(int(jsonGamesFromYear), int(jsonGamesToYear))
}

func generateJSONGamesSingle(gameID string) error {
	builder := pagejson.NewGameBuilder(jsonGamesProcessedDir, jsonGamesVideosCache).SetGamebookParser(gamebookParser)
	paths := pagejson.NewPagePaths(jsonGamesOutputDir)

	season, err := util.NFLSeasonFromGameID(gameID)
	if err != nil {
		return fmt.Errorf("detect NFL season for game %s: %w", gameID, err)
	}

	// Load processed game file
	gameFile := filepath.Join(jsonGamesProcessedDir, "games", fmt.Sprintf("%d", season), fmt.Sprintf("%s.json", gameID))

	log.Printf("Generating JSON game for %s from %s", gameID, gameFile)

	doc, err := builder.BuildFromFile(gameFile, gameID)
	if err != nil {
		return fmt.Errorf("build game JSON for %s: %w", gameID, err)
	}

	// Encode to JSON
	data, err := pagejson.EncodeToJSON(doc)
	if err != nil {
		return fmt.Errorf("encode game JSON for %s: %w", gameID, err)
	}

	// Write to output file
	outputPath := paths.GameOutputPath(gameID)
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory %s: %w", outputDir, err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write game JSON to %s: %w", outputPath, err)
	}

	log.Printf("Generated JSON game: %s", outputPath)
	return nil
}

func generateJSONGamesSeason(season int) error {
	builder := pagejson.NewGameBuilder(jsonGamesProcessedDir, jsonGamesVideosCache).SetGamebookParser(gamebookParser)
	paths := pagejson.NewPagePaths(jsonGamesOutputDir)

	seasonDir := filepath.Join(jsonGamesProcessedDir, "games", fmt.Sprintf("%d", season))
	log.Printf("Generating JSON for all games in season %d from %s", season, seasonDir)

	entries, err := os.ReadDir(seasonDir)
	if err != nil {
		return fmt.Errorf("read games directory %s: %w", seasonDir, err)
	}

	successCount := 0
	errorCount := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		gameID := strings.TrimSuffix(entry.Name(), ".json")
		gameFile := filepath.Join(seasonDir, entry.Name())

		doc, err := builder.BuildFromFile(gameFile, gameID)
		if err != nil {
			log.Printf("Warning: failed to generate game JSON for %s: %v", gameID, err)
			errorCount++
			continue
		}

		data, err := pagejson.EncodeToJSON(doc)
		if err != nil {
			log.Printf("Warning: failed to encode game JSON for %s: %v", gameID, err)
			errorCount++
			continue
		}

		outputPath := paths.GameOutputPath(gameID)
		outputDir := filepath.Dir(outputPath)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			log.Printf("Warning: failed to create directory for %s: %v", gameID, err)
			errorCount++
			continue
		}

		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			log.Printf("Warning: failed to write game JSON for %s: %v", gameID, err)
			errorCount++
			continue
		}

		successCount++
	}

	log.Printf("JSON games generation for season %d complete: %d successful, %d errors", season, successCount, errorCount)
	return nil
}

func generateJSONGamesRange(fromSeason, toSeason int) error {
	builder := pagejson.NewGameBuilder(jsonGamesProcessedDir, jsonGamesVideosCache).SetGamebookParser(gamebookParser)
	paths := pagejson.NewPagePaths(jsonGamesOutputDir)

	log.Printf("Generating JSON for all games in seasons %d-%d", fromSeason, toSeason)

	totalSuccess := 0
	totalErrors := 0

	for season := fromSeason; season <= toSeason; season++ {
		seasonDir := filepath.Join(jsonGamesProcessedDir, "games", fmt.Sprintf("%d", season))

		entries, err := os.ReadDir(seasonDir)
		if err != nil {
			log.Printf("Warning: failed to read season %d directory: %v", season, err)
			totalErrors++
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}

			gameID := strings.TrimSuffix(entry.Name(), ".json")
			gameFile := filepath.Join(seasonDir, entry.Name())

			doc, err := builder.BuildFromFile(gameFile, gameID)
			if err != nil {
				log.Printf("Warning: failed to generate game JSON for %s: %v", gameID, err)
				totalErrors++
				continue
			}

			data, err := pagejson.EncodeToJSON(doc)
			if err != nil {
				log.Printf("Warning: failed to encode game JSON for %s: %v", gameID, err)
				totalErrors++
				continue
			}

			outputPath := paths.GameOutputPath(gameID)
			outputDir := filepath.Dir(outputPath)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				log.Printf("Warning: failed to create directory for %s: %v", gameID, err)
				totalErrors++
				continue
			}

			if err := os.WriteFile(outputPath, data, 0644); err != nil {
				log.Printf("Warning: failed to write game JSON for %s: %v", gameID, err)
				totalErrors++
				continue
			}

			totalSuccess++
		}
	}

	log.Printf("JSON games generation for seasons %d-%d complete: %d successful, %d errors", fromSeason, toSeason, totalSuccess, totalErrors)
	return nil
}
