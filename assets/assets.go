// Package assets defines how remote files (such as team logos) are stored
// locally. Only the contract lives here: downloading from third-party hosts is
// left to the caller, so this library never ships a downloader.
package assets

import "context"

// Downloader stores the file at url in dstPath unless it already exists.
type Downloader interface {
	EnsureDownloaded(ctx context.Context, url, dstPath string) error
}
