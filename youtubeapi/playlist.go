package youtubeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// PlaylistItem represents a single video in a YouTube playlist.
type PlaylistItem struct {
	VideoID     string `json:"video_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	PublishedAt string `json:"published_at"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
}

// PlaylistPage holds one page of playlist results plus a token for the next page.
type PlaylistPage struct {
	Items         []PlaylistItem `json:"items"`
	NextPageToken string         `json:"next_page_token,omitempty"`
	TotalResults  int            `json:"total_results"`
}

// ChannelInfo holds resolved channel metadata.
type ChannelInfo struct {
	ChannelID         string `json:"channel_id"`
	Title             string `json:"title"`
	UploadsPlaylistID string `json:"uploads_playlist_id"`
}

// GetChannelInfo resolves a channel ID to its uploads playlist ID and title.
// Costs 1 quota unit.
func (c *Client) GetChannelInfo(ctx context.Context, channelID string) (*ChannelInfo, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(channelID) == "" {
		return nil, fmt.Errorf("channel ID is empty")
	}

	u, err := url.Parse(strings.TrimRight(c.baseURL(), "/") + "/channels")
	if err != nil {
		return nil, fmt.Errorf("build channels url: %w", err)
	}
	q := u.Query()
	q.Set("part", "snippet,contentDetails")
	q.Set("id", channelID)
	q.Set("fields", "items(id,snippet(title),contentDetails(relatedPlaylists(uploads)))")
	q.Set("key", c.APIKey)
	u.RawQuery = q.Encode()

	resp, err := c.do(ctx, u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, parseAPIError(resp)
	}

	var payload channelsListResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode channels response: %w", err)
	}
	if len(payload.Items) == 0 {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	item := payload.Items[0]
	return &ChannelInfo{
		ChannelID:         strings.TrimSpace(item.ID),
		Title:             strings.TrimSpace(item.Snippet.Title),
		UploadsPlaylistID: strings.TrimSpace(item.ContentDetails.RelatedPlaylists.Uploads),
	}, nil
}

// ListPlaylistPage fetches one page of items from a playlist.
// Costs 1 quota unit. Use pageToken="" for the first page.
func (c *Client) ListPlaylistPage(ctx context.Context, playlistID, pageToken string, maxResults int) (*PlaylistPage, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(playlistID) == "" {
		return nil, fmt.Errorf("playlist ID is empty")
	}
	if maxResults <= 0 {
		maxResults = 50
	}
	if maxResults > 50 {
		maxResults = 50
	}

	u, err := url.Parse(strings.TrimRight(c.baseURL(), "/") + "/playlistItems")
	if err != nil {
		return nil, fmt.Errorf("build playlistItems url: %w", err)
	}
	q := u.Query()
	q.Set("part", "snippet")
	q.Set("playlistId", playlistID)
	q.Set("maxResults", fmt.Sprintf("%d", maxResults))
	q.Set("fields", "nextPageToken,pageInfo(totalResults),items(snippet(title,description,publishedAt,channelId,channelTitle,resourceId(videoId)))")
	q.Set("key", c.APIKey)
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	u.RawQuery = q.Encode()

	resp, err := c.do(ctx, u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, parseAPIError(resp)
	}

	var payload playlistItemsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode playlistItems response: %w", err)
	}

	page := &PlaylistPage{
		NextPageToken: strings.TrimSpace(payload.NextPageToken),
		TotalResults:  payload.PageInfo.TotalResults,
	}

	for _, item := range payload.Items {
		videoID := strings.TrimSpace(item.Snippet.ResourceID.VideoID)
		if videoID == "" {
			continue
		}
		page.Items = append(page.Items, PlaylistItem{
			VideoID:     videoID,
			Title:       strings.TrimSpace(item.Snippet.Title),
			Description: strings.TrimSpace(item.Snippet.Description),
			PublishedAt: strings.TrimSpace(item.Snippet.PublishedAt),
			ChannelID:   strings.TrimSpace(item.Snippet.ChannelID),
			ChannelName: strings.TrimSpace(item.Snippet.ChannelTitle),
		})
	}

	return page, nil
}

// --- Response models ---

type channelsListResponse struct {
	Items []channelsListItem `json:"items"`
}

type channelsListItem struct {
	ID      string `json:"id"`
	Snippet struct {
		Title string `json:"title"`
	} `json:"snippet"`
	ContentDetails struct {
		RelatedPlaylists struct {
			Uploads string `json:"uploads"`
		} `json:"relatedPlaylists"`
	} `json:"contentDetails"`
}

type playlistItemsResponse struct {
	NextPageToken string `json:"nextPageToken"`
	PageInfo      struct {
		TotalResults int `json:"totalResults"`
	} `json:"pageInfo"`
	Items []playlistItemsItem `json:"items"`
}

type playlistItemsItem struct {
	Snippet struct {
		Title        string `json:"title"`
		Description  string `json:"description"`
		PublishedAt  string `json:"publishedAt"`
		ChannelID    string `json:"channelId"`
		ChannelTitle string `json:"channelTitle"`
		ResourceID   struct {
			VideoID string `json:"videoId"`
		} `json:"resourceId"`
	} `json:"snippet"`
}
