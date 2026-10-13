package nflcom

import "time"

func BuildScheduleEnvelope(season int, url string, cacheFile string, data ScheduleData) Envelope[ScheduleData] {
	return Envelope[ScheduleData]{
		Schema:      SchemaInfo{Name: SeasonSchemaName, Version: SeasonSchemaVersion},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Source:      SourceInfo{Site: SiteName, URL: url, CacheFile: cacheFile},
		Team:        TeamInfo{ID: "mia", Name: "Miami Dolphins"},
		Season:      season,
		Data:        data,
	}
}

func BuildGameEnvelope(season int, url string, cacheFile string, data GameData) Envelope[GameData] {
	return Envelope[GameData]{
		Schema:      SchemaInfo{Name: GameSchemaName, Version: GameSchemaVersion},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Source:      SourceInfo{Site: SiteName, URL: url, CacheFile: cacheFile},
		Team:        TeamInfo{ID: "mia", Name: "Miami Dolphins"},
		Season:      season,
		Data:        data,
	}
}
