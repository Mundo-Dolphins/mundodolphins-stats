package youtubeapi

import (
	"errors"
	"fmt"
	"strings"
)

type GoogleAPIError struct {
	StatusCode int
	Reasons    []string
	Message    string
}

func (e *GoogleAPIError) Error() string {
	if e == nil {
		return "google api error"
	}
	if len(e.Reasons) == 0 {
		return fmt.Sprintf("youtube api error: status=%d message=%s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("youtube api error: status=%d reasons=%s message=%s", e.StatusCode, strings.Join(e.Reasons, ","), e.Message)
}

func IsQuotaOrRateLimit(err error) bool {
	var apiErr *GoogleAPIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.StatusCode == 429 {
		return true
	}
	for _, reason := range apiErr.Reasons {
		switch reason {
		case "quotaExceeded", "dailyLimitExceeded", "rateLimitExceeded", "userRateLimitExceeded":
			return true
		}
	}
	return false
}
