package youtubeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// maxResponseBytes bounds how much of an API response is read.
const maxResponseBytes = 8 << 20

// defaultHTTPClient is used when the caller provides none; unlike
// http.DefaultClient it has a timeout.
var defaultHTTPClient = &http.Client{Timeout: 15 * time.Second}

const defaultBaseURL = "https://www.googleapis.com/youtube/v3"

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client wraps the YouTube Data API calls used by the video pipeline.
type Client struct {
	APIKey     string
	HTTPClient httpDoer
	BaseURL    string
}

// SearchResult is a small legacy result shape kept for tests and local mocks.
type SearchResult struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	DurationSec int    `json:"duration_seconds"`
}

func NewClient(apiKey string, clients ...httpDoer) *Client {
	client := httpDoer(defaultHTTPClient)
	if len(clients) > 0 && clients[0] != nil {
		client = clients[0]
	}
	return &Client{
		APIKey:     strings.TrimSpace(apiKey),
		HTTPClient: client,
		BaseURL:    defaultBaseURL,
	}
}

func (c *Client) validate() error {
	if c == nil {
		return fmt.Errorf("youtube api client is nil")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("youtube api key is empty")
	}
	if c.HTTPClient == nil {
		c.HTTPClient = defaultHTTPClient
	}
	return nil
}

func (c *Client) baseURL() string {
	if strings.TrimSpace(c.BaseURL) == "" {
		return defaultBaseURL
	}
	return strings.TrimSpace(c.BaseURL)
}

func (c *Client) do(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, c.redact(err)
	}
	return resp, nil
}

// redact removes the API key from err: the key travels in the query string and
// net/http errors embed the full request URL.
func (c *Client) redact(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	if key := strings.TrimSpace(c.APIKey); key != "" {
		if msg := err.Error(); strings.Contains(msg, key) {
			return errors.New(strings.ReplaceAll(msg, key, "***"))
		}
	}
	return err
}

func (c *Client) SearchVideoIDs(ctx context.Context, query string, maxResults int) ([]string, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if maxResults <= 0 {
		maxResults = 25
	}
	if maxResults > 50 {
		maxResults = 50
	}

	u, err := url.Parse(strings.TrimRight(c.baseURL(), "/") + "/search")
	if err != nil {
		return nil, fmt.Errorf("build search url: %w", err)
	}
	q := u.Query()
	q.Set("part", "snippet")
	q.Set("type", "video")
	q.Set("q", query)
	q.Set("maxResults", strconv.Itoa(maxResults))
	q.Set("fields", "items(id(videoId))")
	q.Set("key", c.APIKey)
	u.RawQuery = q.Encode()

	resp, err := c.do(ctx, u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var payload searchListResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	ids := make([]string, 0, len(payload.Items))
	seen := make(map[string]bool, len(payload.Items))
	for _, item := range payload.Items {
		id := strings.TrimSpace(item.ID.VideoID)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (c *Client) HydrateVideos(ctx context.Context, ids []string) ([]VideoCandidate, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	var out []VideoCandidate
	for _, chunk := range chunkIDs(ids, 50) {
		u, err := url.Parse(strings.TrimRight(c.baseURL(), "/") + "/videos")
		if err != nil {
			return nil, fmt.Errorf("build videos url: %w", err)
		}
		q := u.Query()
		q.Set("part", "snippet,contentDetails,statistics,status")
		q.Set("id", strings.Join(chunk, ","))
		q.Set("fields", "items(id,snippet(title,description,channelTitle,channelId,publishedAt),contentDetails(duration),statistics(viewCount),status(embeddable))")
		q.Set("key", c.APIKey)
		u.RawQuery = q.Encode()

		resp, err := c.do(ctx, u.String())
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, parseAPIError(resp)
		}

		var payload videosListResponse
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&payload); err != nil {
			return nil, fmt.Errorf("decode videos response: %w", err)
		}
		for _, item := range payload.Items {
			duration, err := parseISO8601Duration(item.ContentDetails.Duration)
			if err != nil {
				continue
			}
			viewCount, _ := strconv.ParseUint(item.Statistics.ViewCount, 10, 64)
			out = append(out, VideoCandidate{
				ID:          strings.TrimSpace(item.ID),
				URL:         "https://www.youtube.com/watch?v=" + strings.TrimSpace(item.ID),
				Title:       strings.TrimSpace(item.Snippet.Title),
				Description: strings.TrimSpace(item.Snippet.Description),
				Channel:     strings.TrimSpace(item.Snippet.Channel),
				ChannelID:   strings.TrimSpace(item.Snippet.ChannelID),
				Duration:    duration,
				Embeddable:  item.Status.Embeddable,
				ViewCount:   viewCount,
				PublishedAt: strings.TrimSpace(item.Snippet.PublishedAt),
			})
		}
	}

	return out, nil
}

func (c *Client) CheckExistence(ctx context.Context, ids []string) (map[string]bool, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	existing := make(map[string]bool, len(ids))
	if len(ids) == 0 {
		return existing, nil
	}

	for _, chunk := range chunkIDs(ids, 50) {
		u, err := url.Parse(strings.TrimRight(c.baseURL(), "/") + "/videos")
		if err != nil {
			return nil, fmt.Errorf("build videos url: %w", err)
		}
		q := u.Query()
		q.Set("part", "id")
		q.Set("id", strings.Join(chunk, ","))
		q.Set("fields", "items(id)")
		q.Set("key", c.APIKey)
		u.RawQuery = q.Encode()

		resp, err := c.do(ctx, u.String())
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, parseAPIError(resp)
		}

		var payload videosListResponse
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&payload); err != nil {
			return nil, fmt.Errorf("decode videos response: %w", err)
		}
		for _, item := range payload.Items {
			id := strings.TrimSpace(item.ID)
			if id != "" {
				existing[id] = true
			}
		}
	}

	return existing, nil
}

func parseAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))

	var payload googleErrorEnvelope
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error.Code != 0 {
		apiErr := &GoogleAPIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(payload.Error.Message),
		}
		for _, item := range payload.Error.Errors {
			if reason := strings.TrimSpace(item.Reason); reason != "" {
				apiErr.Reasons = append(apiErr.Reasons, reason)
			}
		}
		return apiErr
	}

	return &GoogleAPIError{
		StatusCode: resp.StatusCode,
		Message:    strings.TrimSpace(string(body)),
	}
}

var isoDurationRE = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

func parseISO8601Duration(value string) (time.Duration, error) {
	matches := isoDurationRE.FindStringSubmatch(strings.TrimSpace(value))
	if matches == nil {
		return 0, fmt.Errorf("invalid ISO 8601 duration %q", value)
	}

	var total time.Duration
	units := []time.Duration{time.Hour, time.Minute, time.Second}
	for i, unit := range units {
		part := matches[i+1]
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return 0, err
		}
		total += time.Duration(n) * unit
	}
	if total == 0 {
		return 0, fmt.Errorf("empty ISO 8601 duration %q", value)
	}
	return total, nil
}

func chunkIDs(ids []string, size int) [][]string {
	if len(ids) == 0 {
		return nil
	}
	if size <= 0 {
		size = len(ids)
	}

	chunks := make([][]string, 0, (len(ids)+size-1)/size)
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[start:end])
	}
	return chunks
}
