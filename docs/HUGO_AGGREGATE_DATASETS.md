# Hugo Aggregate Datasets

This document tracks reusable aggregate JSON datasets generated for `mundodolphins.es`.
They are data contracts for Hugo consumers, not presentation templates.

## Data Audit

Available offline inputs:

- `processed/<season>.json`
  - Season envelope with schema, generated timestamp, source, team, season, and page payloads.
  - `data.main.info` includes season summary text such as record, coach, points, and ranking when present.
  - `data.main.division_name`, `division_rank`, `division_link`, `staff`, `principal`, and `logo` are structured when parsed.
  - `data.<page>.tables` contains normalized PFR table rows for roster, schedule, stats, draft, injuries, trades, and other supported pages when available.
- `processed/games/<season>/<game_id>.json`
  - Boxscore envelope with schema, generated timestamp, source, game id, scorebox, linescore, week, YouTube block, and parsed tables.
  - `data.scorebox.teams` gives visitor/home team names, ids or links, scores, records, coaches, and logos when present.
  - `data.scorebox.meta.date` and `game_id` allow deterministic game dates and NFL season grouping.
  - `data.week` allows regular season vs playoff classification when it is numeric or contains playoff labels.
  - `data.tables` contains team stats, scoring, player stats, officials, drives, snap counts, and advanced stats when present.
- `i18n`
  - Spanish labels exist for sections, field labels, table headers, months, weather, and scoring descriptions.
  - Team names are currently returned unchanged.

Relations derivable without network:

- Season to games by `processed/games/<season>`.
- Game to opponent from `scorebox.teams` by locating Miami Dolphins.
- Home/away from scorebox order.
- Result, margin, points for, and points against from scorebox scores.
- Regular season/playoffs only when `week` is numeric or has playoff wording.
- Public game path can be generated from visitor, home, and normalized date with the existing game slug logic.

## Candidate Datasets

1. Rival index
   - Impact: high for navigation, rivalry pages, and historical comparisons.
   - Cost: low because it depends on game scoreboxes, which are already structured.
   - Risk: low; skips games without a valid two-team scorebox.

2. Season comparison index
   - Impact: high for sortable season tables and compare tools.
   - Cost: medium because season records and rankings are mostly available, but summary fields vary across eras.
   - Risk: medium; final outcome/playoff result needs careful derivation from schedule rows.

3. Historical leaders
   - Impact: high for editorial evergreen pages.
   - Cost: medium to high because player-table normalization and era-specific table keys need more validation.
   - Risk: higher; career totals inside the available dataset are only as complete as parsed seasons.

The first implemented dataset is the rival index.

## Rival Index Contract

Output path:

```text
output/data/historical/rivals/index.json
```

Schema:

```json
{
  "name": "mundodolphins.hugo.rival-index",
  "version": "1.0.0"
}
```

Top-level fields:

- `schema`: schema name and version.
- `generated_at`: RFC3339 timestamp. Tests inject a fixed clock.
- `data.team`: canonical team reference for Miami.
- `data.range`: requested season range.
- `data.totals`: aggregate W-L-T, points, and game-type counts across all included games.
- `data.rivals`: sorted by opponent name, then opponent id.

Rival fields:

- `opponent`: `id` and `name`.
- `summary`: aggregate games, W-L-T, points for/against, point differential, regular season games, playoff games, unknown games when any, first game date, and last game date.
- `by_season`: deterministic ascending season split with the same core aggregate fields.
- `latest_games`: latest five games in descending date order.

Game summary fields:

- `game_id`
- `season`
- `date`
- `week`
- `game_type`: `regular_season`, `playoffs`, or `unknown`.
- `home_away`: `home` or `away` from Miami's perspective.
- `opponent_id`
- `opponent_name`
- `points_for`
- `points_against`
- `result`: `W`, `L`, or `T`.
- `margin`
- `path`: generated public game path when names and date are present.

Generation commands:

```bash
dolphins-stats json rivals --from-year 1966 --to-year 2026
dolphins-stats generate hugo-data --type rivals --from-year 1966 --to-year 2026
```

Hugo usage examples after copying `output/data/historical` to the web repo as `data/historical`:

```go-html-template
{{ $rivals := site.Data.historical.rivals.index.data.rivals }}
{{ range $rivals }}
  {{ .opponent.name }}: {{ .summary.wins }}-{{ .summary.losses }}-{{ .summary.ties }}
{{ end }}
```

```go-html-template
{{ $bills := where site.Data.historical.rivals.index.data.rivals "opponent.id" "buf" }}
{{ with index $bills 0 }}
  {{ range .latest_games }}
    {{ .date }} {{ .result }} {{ .points_for }}-{{ .points_against }}
  {{ end }}
{{ end }}
```
