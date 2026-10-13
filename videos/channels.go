package videos

// ChannelConfig defines a YouTube channel candidate to scan for game videos.
type ChannelConfig struct {
	ID   string // YouTube channel ID (e.g. "UCT8jP2abCaGYwFl_LQUTAOQ")
	Name string // Human-readable name (e.g. "Vintage Pro Football")
}

const (
	DefaultMinChannelVideos = 2
	DefaultMinChannelScore  = 0.3
)

// BlockedChannelIDs contains channel IDs that should NEVER be used for game matching.
// These channels produce noise: highlights compilations, analysis, news, etc.
var BlockedChannelIDs = map[string]string{
	"UCDVYQ4Zhbm3S2dlz7P1GBDg": "NFL",             // too much noise: highlights, Top 10, previews
	"UCYTBCo_CVBXblEOalwGPJqg": "ESPN",            // analysis, not full games
	"UCuoTKZOw9rmx_sDq498LIbA": "NFL Films",       // compilations, not individual games
	"UCBs4RxnCfMxyIvtqrLl70xA": "CBS Sports",      // news and analysis
	"UCVjOEbMjJmdcOh0r2rhJa-A": "FOX Sports",      // news and analysis
	"UCvAvFl2OGsuDSoOo93Yrauw": "Pat McAfee Show", // podcast/analysis
	"UCLk-mBF3_xMvMvwB5eYJtiA": "The Ringer",      // analysis
}

// IsBlockedChannel returns true if a channel ID is in the blocklist.
func IsBlockedChannel(channelID string) bool {
	_, blocked := BlockedChannelIDs[channelID]
	return blocked
}
