package videos

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/Mundo-Dolphins/mundodolphins-stats/youtubeapi"
	_ "modernc.org/sqlite"
)

type ScorerReportConfig struct {
	GamesDir       string
	CacheFile      string
	DiscoveryDB    string
	MinDurationSec int
	ScoreThreshold int
	ExampleLimit   int
	FromYear       int
	ToYear         int
}

type ScorerReport struct {
	Summary               ScorerReportSummary      `json:"summary"`
	Reasons               []ScorerReasonCount      `json:"reasons"`
	ApprovedIssues        []ScorerCandidateFinding `json:"approved_issues,omitempty"`
	AutoApproveCandidates []ScorerCandidateFinding `json:"auto_approve_candidates,omitempty"`
	DiscardedExamples     []ScorerCandidateFinding `json:"discarded_examples,omitempty"`
	ZeroCandidateGames    []ScorerGameFinding      `json:"zero_candidate_games,omitempty"`
	DiscoveryByGame       []ScorerGameFinding      `json:"discovery_by_game,omitempty"`
	ApprovedWithoutVideo  []ScorerApprovedNoVideo  `json:"approved_without_video,omitempty"`
}

type ScorerReportSummary struct {
	GamesLoaded             int `json:"games_loaded"`
	CacheItems              int `json:"cache_items"`
	ReviewedGames           int `json:"reviewed_games"`
	ApprovedGames           int `json:"approved_games"`
	ApprovedWithoutVideos   int `json:"approved_without_videos"`
	ApprovedVideos          int `json:"approved_videos"`
	ApprovedVideosKept      int `json:"approved_videos_kept"`
	ApprovedVideosRejected  int `json:"approved_videos_rejected"`
	ApprovedBelowThreshold  int `json:"approved_below_threshold"`
	ApprovedAutoEligible    int `json:"approved_auto_eligible"`
	DiscoverySearches       int `json:"discovery_searches"`
	DiscoveryPairs          int `json:"discovery_pairs"`
	DiscoveryCandidatesKept int `json:"discovery_candidates_kept"`
	DiscoveryAutoAcceptable int `json:"discovery_auto_acceptable"`
	DiscoveryAutoEligible   int `json:"discovery_auto_eligible"`
	DiscoveryGames          int `json:"discovery_games"`
	DiscoveryGamesWithKept  int `json:"discovery_games_with_kept"`
	DiscoveryGamesZeroKept  int `json:"discovery_games_zero_kept"`
}

type ScorerReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type ScorerCandidateFinding struct {
	GameID                 string `json:"game_id"`
	VideoID                string `json:"video_id,omitempty"`
	Title                  string `json:"title,omitempty"`
	Channel                string `json:"channel,omitempty"`
	DurationSec            int    `json:"duration_seconds,omitempty"`
	Score                  int    `json:"score"`
	Reason                 string `json:"reason"`
	Decision               string `json:"decision,omitempty"`
	VideoKind              string `json:"video_kind,omitempty"`
	DateMatch              string `json:"date_match,omitempty"`
	WeekMatch              string `json:"week_match,omitempty"`
	ClassificationDecision string `json:"classification_decision,omitempty"`
	Sources                string `json:"sources,omitempty"`
}

type ScorerGameFinding struct {
	GameID         string `json:"game_id"`
	Raw            int    `json:"raw"`
	Kept           int    `json:"kept"`
	Rejected       int    `json:"rejected"`
	Approved       bool   `json:"approved,omitempty"`
	ApprovedVideos int    `json:"approved_videos,omitempty"`
}

type ScorerApprovedNoVideo struct {
	GameID string `json:"game_id"`
	Status string `json:"status"`
}

type candidateEvaluation struct {
	Score          int
	Reason         string
	Decision       ClassificationDecision
	VideoKind      LegacyVideoKind
	DateMatch      DateMatchType
	WeekMatch      DateMatchType
	Classification ClassificationResult
	Kept           bool
}

type discoveryCandidateRow struct {
	GameID  string
	Sources string
	Candidate
}

func BuildScorerReport(ctx context.Context, cfg ScorerReportConfig) (*ScorerReport, error) {
	if cfg.GamesDir == "" {
		return nil, fmt.Errorf("games dir is empty")
	}
	if cfg.CacheFile == "" {
		return nil, fmt.Errorf("cache file is empty")
	}
	if cfg.DiscoveryDB == "" {
		return nil, fmt.Errorf("discovery db is empty")
	}
	if cfg.MinDurationSec <= 0 {
		cfg.MinDurationSec = 300
	}
	if cfg.ScoreThreshold <= 0 {
		cfg.ScoreThreshold = 60
	}
	if cfg.ExampleLimit <= 0 {
		cfg.ExampleLimit = 20
	}

	games, err := NewJSONGameLoader(cfg.GamesDir).LoadGames(cfg.FromYear, cfg.ToYear)
	if err != nil {
		return nil, err
	}
	gamesByID := make(map[string]GameRef, len(games))
	for _, game := range games {
		gamesByID[game.GameID] = game
	}

	cf, err := NewFileCache(cfg.CacheFile).Load()
	if err != nil {
		return nil, err
	}
	rows, searches, err := loadDiscoveryCandidateRows(ctx, cfg.DiscoveryDB)
	if err != nil {
		return nil, err
	}

	report := &ScorerReport{}
	report.Summary.GamesLoaded = len(games)
	report.Summary.CacheItems = len(cf.Items)
	report.Summary.DiscoverySearches = searches
	report.Summary.DiscoveryPairs = len(rows)

	scorer := NewScorer(cfg.MinDurationSec, cfg.ScoreThreshold)
	approvedByGame := map[string]int{}
	for gameID, item := range cf.Items {
		if item == nil {
			continue
		}
		if IsDiscoveryCompleteStatus(item.Status) {
			report.Summary.ReviewedGames++
		}
		if item.Status == StatusApproved {
			report.Summary.ApprovedGames++
		}
		if item.Status == StatusApproved && len(item.Videos) == 0 {
			report.Summary.ApprovedWithoutVideos++
			if len(report.ApprovedWithoutVideo) < cfg.ExampleLimit {
				report.ApprovedWithoutVideo = append(report.ApprovedWithoutVideo, ScorerApprovedNoVideo{GameID: gameID, Status: item.Status})
			}
		}
		if !IsPublishableStatus(item.Status) {
			continue
		}
		game, ok := gamesByID[gameID]
		if !ok {
			continue
		}
		for _, video := range item.Videos {
			report.Summary.ApprovedVideos++
			approvedByGame[gameID]++
			candidate := candidateFromVideo(video)
			eval := evaluateCandidateForScorerReport(game, candidate, scorer)
			if eval.Kept {
				report.Summary.ApprovedVideosKept++
			} else {
				report.Summary.ApprovedVideosRejected++
			}
			if eval.Score < cfg.ScoreThreshold {
				report.Summary.ApprovedBelowThreshold++
			}
			if strictAutoApproveCandidate(game, candidate, cfg.MinDurationSec, cfg.ScoreThreshold) {
				report.Summary.ApprovedAutoEligible++
			}
			if (!eval.Kept || eval.Score < cfg.ScoreThreshold) && len(report.ApprovedIssues) < cfg.ExampleLimit {
				report.ApprovedIssues = append(report.ApprovedIssues, scorerCandidateFinding(gameID, candidate, eval, "approved"))
			}
		}
	}

	reasonCounts := map[string]int{}
	byGame := map[string]*ScorerGameFinding{}
	for _, row := range rows {
		game, ok := gamesByID[row.GameID]
		if !ok {
			reasonCounts["missing_game"]++
			continue
		}
		stat := byGame[row.GameID]
		if stat == nil {
			stat = &ScorerGameFinding{GameID: row.GameID, ApprovedVideos: approvedByGame[row.GameID]}
			stat.Approved = approvedByGame[row.GameID] > 0
			byGame[row.GameID] = stat
		}
		stat.Raw++
		eval := evaluateCandidateForScorerReport(game, row.Candidate, scorer)
		reasonCounts[eval.Reason]++
		if eval.Kept {
			stat.Kept++
			report.Summary.DiscoveryCandidatesKept++
			if eval.Decision == DecisionAccept {
				report.Summary.DiscoveryAutoAcceptable++
				if strictAutoApproveCandidate(game, row.Candidate, cfg.MinDurationSec, cfg.ScoreThreshold) {
					report.Summary.DiscoveryAutoEligible++
				}
				if approvedByGame[row.GameID] == 0 && strictAutoApproveCandidate(game, row.Candidate, cfg.MinDurationSec, cfg.ScoreThreshold) && len(report.AutoApproveCandidates) < cfg.ExampleLimit {
					report.AutoApproveCandidates = append(report.AutoApproveCandidates, scorerCandidateFinding(row.GameID, row.Candidate, eval, row.Sources))
				}
			}
			continue
		}
		stat.Rejected++
		if shouldKeepDiscardedExample(eval.Reason) && len(report.DiscardedExamples) < cfg.ExampleLimit {
			report.DiscardedExamples = append(report.DiscardedExamples, scorerCandidateFinding(row.GameID, row.Candidate, eval, row.Sources))
		}
	}

	report.Reasons = sortedReasonCounts(reasonCounts)
	report.DiscoveryByGame = sortedGameFindings(byGame, cfg.ExampleLimit, false)
	report.ZeroCandidateGames = sortedGameFindings(byGame, cfg.ExampleLimit, true)
	report.Summary.DiscoveryGames = len(byGame)
	for _, stat := range byGame {
		if stat.Kept > 0 {
			report.Summary.DiscoveryGamesWithKept++
		} else if stat.Raw > 0 {
			report.Summary.DiscoveryGamesZeroKept++
		}
	}
	return report, nil
}

func loadDiscoveryCandidateRows(ctx context.Context, path string) ([]discoveryCandidateRow, int, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, 0, fmt.Errorf("open discovery db: %w", err)
	}
	defer db.Close()

	var searches int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM searches`).Scan(&searches); err != nil {
		return nil, 0, fmt.Errorf("count discovery searches: %w", err)
	}

	rows, err := db.QueryContext(ctx, `
SELECT s.game_id,
       v.video_id,
       v.url,
       v.title,
       v.channel,
       v.duration_text,
       v.duration_seconds,
       v.snippet,
       v.embeddable,
       group_concat(DISTINCT s.source)
FROM search_results sr
JOIN searches s ON s.id = sr.search_id
JOIN videos v ON v.video_id = sr.video_id
WHERE s.game_id <> ''
GROUP BY s.game_id, v.video_id
ORDER BY s.game_id, max(s.searched_at) DESC`)
	if err != nil {
		return nil, 0, fmt.Errorf("query discovery candidates: %w", err)
	}
	defer rows.Close()

	out := make([]discoveryCandidateRow, 0)
	for rows.Next() {
		var row discoveryCandidateRow
		var title, channel, durationText, snippet, sources sql.NullString
		var durationSec sql.NullInt64
		var embeddable sql.NullInt64
		if err := rows.Scan(&row.GameID, &row.VideoID, &row.URL, &title, &channel, &durationText, &durationSec, &snippet, &embeddable, &sources); err != nil {
			return nil, 0, fmt.Errorf("scan discovery candidate: %w", err)
		}
		row.Title = title.String
		row.Channel = channel.String
		row.DurationText = durationText.String
		row.DurationSec = int(durationSec.Int64)
		row.Snippet = snippet.String
		row.Sources = sources.String
		if embeddable.Valid {
			value := embeddable.Int64 != 0
			row.Embeddable = &value
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate discovery candidates: %w", err)
	}
	return out, searches, nil
}

func evaluateCandidateForScorerReport(game GameRef, c Candidate, scorer Scorer) candidateEvaluation {
	if scorer.MinDurationSec <= 0 {
		scorer.MinDurationSec = 300
	}
	if scorer.ScoreThreshold <= 0 {
		scorer.ScoreThreshold = 60
	}
	legacyEval := scorer.Evaluate(game, c)
	eval := candidateEvaluation{
		Score:     legacyEval.Score,
		Reason:    legacyEval.Reason,
		Decision:  legacyEval.Decision,
		VideoKind: legacyEval.VideoKind,
		DateMatch: legacyEval.DateMatch,
		WeekMatch: legacyEval.WeekMatch,
	}
	if legacyEval.Decision == DecisionReject {
		return eval
	}
	policy := DefaultClassificationPolicy()
	policy.MinDurationSec = scorer.MinDurationSec
	eval.Classification = ClassifyLegacyCandidate(game, c, policy)
	if eval.Decision == DecisionAccept && eval.Classification.Decision == DecisionReject {
		eval.Reason = "classifier_reject"
		eval.Decision = DecisionReject
		return eval
	}
	eval.Kept = true
	if eval.Reason == "" {
		eval.Reason = "kept"
	}
	return eval
}

func candidateFromVideo(video Video) Candidate {
	return Candidate{
		VideoID:     video.ID,
		URL:         video.URL,
		Title:       video.Title,
		Channel:     video.Channel,
		DurationSec: video.DurationSeconds,
		Embeddable:  video.Embeddable,
	}
}

func scorerCandidateFinding(gameID string, candidate Candidate, eval candidateEvaluation, sources string) ScorerCandidateFinding {
	return ScorerCandidateFinding{
		GameID:                 gameID,
		VideoID:                candidate.VideoID,
		Title:                  candidate.Title,
		Channel:                candidate.Channel,
		DurationSec:            candidate.DurationSec,
		Score:                  eval.Score,
		Reason:                 eval.Reason,
		Decision:               string(eval.Decision),
		VideoKind:              string(eval.VideoKind),
		DateMatch:              string(eval.DateMatch),
		WeekMatch:              string(eval.WeekMatch),
		ClassificationDecision: string(eval.Classification.Decision),
		Sources:                sources,
	}
}

func shouldKeepDiscardedExample(reason string) bool {
	switch reason {
	case "missing_temporal", "wrong_week", "wrong_broadcast_weekday", "classifier_reject", "score_non_positive":
		return true
	default:
		return false
	}
}

func strictAutoApproveCandidate(game GameRef, candidate Candidate, minDurationSec int, threshold int) bool {
	if threshold < 130 {
		threshold = 130
	}
	durationSec := candidateDurationSec(candidate)
	scored := ScoreCandidate(ExtractGameMetadata(game), youtubeapi.VideoCandidate{
		ID:          candidate.VideoID,
		URL:         candidate.URL,
		Title:       candidate.Title,
		Description: candidate.Snippet,
		Channel:     candidate.Channel,
		Duration:    time.Duration(durationSec) * time.Second,
		Embeddable:  candidate.Embeddable == nil || *candidate.Embeddable,
		Score:       candidate.Score,
	}, minDurationSec)
	return ShouldAutoApprove(scored, threshold)
}

func sortedReasonCounts(counts map[string]int) []ScorerReasonCount {
	out := make([]ScorerReasonCount, 0, len(counts))
	for reason, count := range counts {
		out = append(out, ScorerReasonCount{Reason: reason, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Reason < out[j].Reason
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func sortedGameFindings(stats map[string]*ScorerGameFinding, limit int, zeroOnly bool) []ScorerGameFinding {
	out := make([]ScorerGameFinding, 0, len(stats))
	for _, stat := range stats {
		if zeroOnly && !(stat.Raw > 0 && stat.Kept == 0) {
			continue
		}
		out = append(out, *stat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kept == out[j].Kept {
			if out[i].Raw == out[j].Raw {
				return out[i].GameID < out[j].GameID
			}
			return out[i].Raw > out[j].Raw
		}
		if zeroOnly {
			return out[i].Raw > out[j].Raw
		}
		return out[i].Kept < out[j].Kept
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
