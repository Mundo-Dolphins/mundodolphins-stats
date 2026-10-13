package videos

import "context"

type Searcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]Candidate, error)
}

type Cache interface {
	Load() (*CacheFile, error)
	Save(cf *CacheFile) error
	Upsert(gameID string, item *CacheItem) error
	Get(gameID string) (*CacheItem, bool, error)
}

type GameLoader interface {
	LoadGames(yearFrom, yearTo int) ([]GameRef, error)
}
