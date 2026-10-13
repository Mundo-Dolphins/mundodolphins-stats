package cli

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/Mundo-Dolphins/mundodolphins-stats/hugodata"
	"github.com/Mundo-Dolphins/mundodolphins-stats/util"
	"github.com/spf13/cobra"
)

var generateHugoDataCmd = &cobra.Command{
	Use:   "hugo-data",
	Short: "Generate historical JSON data for Hugo",
	Long: `Generate structured historical JSON for the Hugo site from processed JSON files.

Data outputs are written under <output-dir>/data/historical/{seasons,season_stats,games,rivals}.
Editorial articles remain Markdown and can be transformed from articles/*.md when --include-articles is enabled.`,
	RunE: runGenerateHugoData,
}

var (
	hugoYear            int
	hugoFromYear        int
	hugoToYear          int
	hugoOutputDir       string
	hugoAssetsDir       string
	hugoArticlesDir     string
	hugoArticlesOut     string
	hugoGamesDir        string
	hugoDataType        string // "all", "seasons", "season_stats", "games", "rivals"
	hugoGameID          string
	hugoProcessedDir    string
	hugoVideosCache     string
	hugoEnsureAssets    bool
	hugoIncludeArticles bool
)

func init() {
	generateHugoDataCmd.Flags().IntVar(&hugoYear, "year", 0, "Single year to generate (e.g., 2024)")
	generateHugoDataCmd.Flags().IntVar(&hugoFromYear, "from-year", 1966, "Start year for range generation")
	generateHugoDataCmd.Flags().IntVar(&hugoToYear, "to-year", 2026, "End year for range generation")
	generateHugoDataCmd.Flags().StringVar(&hugoOutputDir, "output-dir", "output", "Output base directory; data is written below data/historical")
	generateHugoDataCmd.Flags().StringVar(&hugoAssetsDir, "assets-dir", "", "Assets directory (defaults to <output-dir>/assets when --ensure-assets is set)")
	generateHugoDataCmd.Flags().BoolVar(&hugoIncludeArticles, "include-articles", true, "Generate season articles from articles/*.md into output")
	generateHugoDataCmd.Flags().StringVar(&hugoArticlesDir, "articles-dir", "articles", "Directory with source season articles markdown")
	generateHugoDataCmd.Flags().StringVar(&hugoArticlesOut, "articles-output-dir", "output/historia/articulos", "Output directory for transformed season articles")
	generateHugoDataCmd.Flags().StringVar(&hugoGamesDir, "games-dir", "processed/games", "Directory with processed games JSON for article dates")
	generateHugoDataCmd.Flags().StringVar(&hugoDataType, "type", "all", "Data type: all, seasons, season_stats, games, rivals")
	generateHugoDataCmd.Flags().StringVar(&hugoGameID, "game-id", "", "Generate data for single game ID")
	generateHugoDataCmd.Flags().StringVar(&hugoProcessedDir, "processed-dir", "processed", "Directory containing processed JSON files")
	generateHugoDataCmd.Flags().StringVar(&hugoVideosCache, "videos-cache", "cache/videos/videos.json", "Path to videos cache JSON")
	generateHugoDataCmd.Flags().BoolVar(&hugoEnsureAssets, "ensure-assets", false, "Download missing assets required by generated JSON")

	generateCmd.AddCommand(generateHugoDataCmd)
}

func runGenerateHugoData(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Single game by ID
	if hugoGameID != "" {
		return generateHugoDataForGame(ctx, hugoGameID)
	}

	// Determine year range
	fromYear := hugoFromYear
	toYear := hugoToYear
	if hugoYear > 0 {
		fromYear = hugoYear
		toYear = hugoYear
	}

	log.Printf("Generating Hugo data from %d to %d (type: %s)", fromYear, toYear, hugoDataType)

	for year := fromYear; year <= toYear; year++ {
		if hugoIncludeArticles && hugoDataType != "games" && hugoDataType != "rivals" {
			articleOutputPath, err := generateSeasonArticle(year, hugoArticlesDir, hugoArticlesOut, hugoGamesDir)
			if err != nil {
				log.Printf("Warning: failed to generate season article for %d: %v", year, err)
			} else if articleOutputPath != "" {
				log.Printf("Generated season article for %d -> %s", year, articleOutputPath)
			}
		}

		// Generate seasons data
		if hugoDataType == "all" || hugoDataType == "seasons" {
			if err := generateHugoDataForSeason(ctx, year); err != nil {
				log.Printf("Warning: failed to generate season data for %d: %v", year, err)
			}
		}

		// Generate season stats data
		if hugoDataType == "all" || hugoDataType == "season_stats" {
			if err := generateHugoDataForSeasonStats(ctx, year); err != nil {
				log.Printf("Warning: failed to generate season stats data for %d: %v", year, err)
			}
		}

		// Generate games data (if type is all or games)
		if hugoDataType == "all" || hugoDataType == "games" {
			generateHugoDataForSeasonGames(ctx, year)
		}
	}

	if hugoDataType == "all" || hugoDataType == "rivals" {
		if err := generateHugoDataForRivals(ctx, fromYear, toYear); err != nil {
			if hugoDataType == "rivals" {
				return err
			}
			log.Printf("Warning: failed to generate rival index: %v", err)
		}
	}

	log.Printf("Hugo historical data generation complete for years %d-%d", fromYear, toYear)
	return nil
}

func generateHugoDataForSeason(ctx context.Context, year int) error {
	filename := filepath.Join(hugoProcessedDir, fmt.Sprintf("%d.json", year))
	env, err := hugodata.LoadSeasonFile(filename)
	if err != nil {
		return fmt.Errorf("load season file %s: %w", filename, err)
	}

	if err := hugodata.GenerateSeasonDataWithOptions(
		ctx,
		env,
		hugoOutputDir,
		hugodata.GenerateOptions{
			Assets:          assetsDownloader,
			GamebookParser:  gamebookParser,
			EnsureAssets:    hugoEnsureAssets,
			AssetsDir:       hugoAssetsDir,
			VideosCacheFile: hugoVideosCache,
			ProcessedDir:    hugoProcessedDir,
		},
	); err != nil {
		return fmt.Errorf("generate season data: %w", err)
	}

	log.Printf("Generated Hugo season data for %d", year)
	return nil
}

func generateHugoDataForSeasonStats(ctx context.Context, year int) error {
	filename := filepath.Join(hugoProcessedDir, fmt.Sprintf("%d.json", year))
	env, err := hugodata.LoadSeasonFile(filename)
	if err != nil {
		return fmt.Errorf("load season file %s: %w", filename, err)
	}

	if err := hugodata.GenerateSeasonStatsData(ctx, env, hugoOutputDir); err != nil {
		return fmt.Errorf("generate season stats data: %w", err)
	}

	log.Printf("Generated Hugo season stats data for %d", year)
	return nil
}

func generateHugoDataForSeasonGames(ctx context.Context, year int) {
	// Find all game JSON files for this year
	gamesDir := filepath.Join(hugoProcessedDir, "games", fmt.Sprintf("%d", year))

	entries, err := filepath.Glob(filepath.Join(gamesDir, "*.json"))
	if err != nil {
		// Directory might not exist, skip silently
		return
	}

	for _, gameFile := range entries {
		env, err := hugodata.LoadGameFile(gameFile)
		if err != nil {
			log.Printf("Warning: failed to load game file %s: %v", gameFile, err)
			continue
		}

		if err := hugodata.GenerateGameDataWithOptions(
			ctx,
			env,
			hugoOutputDir,
			hugodata.GenerateOptions{
				Assets:          assetsDownloader,
				GamebookParser:  gamebookParser,
				EnsureAssets:    hugoEnsureAssets,
				AssetsDir:       hugoAssetsDir,
				VideosCacheFile: hugoVideosCache,
				ProcessedDir:    hugoProcessedDir,
			},
		); err != nil {
			log.Printf("Warning: failed to generate game data for %s: %v", env.GameID, err)
			continue
		}
	}

	log.Printf("Generated Hugo game data for season %d", year)
}

func generateHugoDataForGame(ctx context.Context, gameID string) error {
	// Extract season from game ID
	season, err := util.NFLSeasonFromGameID(gameID)
	if err != nil {
		return fmt.Errorf("determine season for game %s: %w", gameID, err)
	}

	gameFile := filepath.Join(hugoProcessedDir, "games", fmt.Sprintf("%d", season), fmt.Sprintf("%s.json", gameID))
	env, err := hugodata.LoadGameFile(gameFile)
	if err != nil {
		return fmt.Errorf("load game file %s: %w", gameFile, err)
	}

	if err := hugodata.GenerateGameDataWithOptions(
		ctx,
		env,
		hugoOutputDir,
		hugodata.GenerateOptions{
			Assets:          assetsDownloader,
			GamebookParser:  gamebookParser,
			EnsureAssets:    hugoEnsureAssets,
			AssetsDir:       hugoAssetsDir,
			VideosCacheFile: hugoVideosCache,
			ProcessedDir:    hugoProcessedDir,
		},
	); err != nil {
		return fmt.Errorf("generate game data: %w", err)
	}

	log.Printf("Generated Hugo game data for %s (season %d)", gameID, season)
	return nil
}

func generateHugoDataForRivals(ctx context.Context, fromYear, toYear int) error {
	if err := hugodata.GenerateRivalIndexData(ctx, hugoProcessedDir, hugoOutputDir, fromYear, toYear); err != nil {
		return fmt.Errorf("generate rival index: %w", err)
	}
	log.Printf("Generated Hugo rival index for seasons %d-%d", fromYear, toYear)
	return nil
}
