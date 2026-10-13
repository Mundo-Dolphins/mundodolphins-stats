package season_stats

type SeasonStatsInput struct {
	Year   int
	Tables map[string]any
}

type SeasonStatsViewModel struct {
	Year int

	TeamPassing      TeamPassingStats
	TeamRushing      TeamRushingStats
	TeamReceiving    TeamReceivingStats
	TeamDefense      TeamDefenseStats
	TeamSpecialTeams TeamSpecialTeamsStats
	TeamScoring      TeamScoringStats

	PlayerPassing   []PlayerPassingRow
	PlayerRushing   []PlayerRushingRow
	PlayerReceiving []PlayerReceivingRow
	PlayerDefense   []PlayerDefenseRow
	PlayerScoring   []PlayerScoringRow

	// Special Teams Player Tables
	KickingPlayers []KickingPlayer
	PuntingPlayers []PuntingPlayer
	ReturnsPlayers []ReturnsPlayer

	// Team-level tables
	TeamConversions TeamConversions
	TeamTDLog       []TDLogEntry
	OppTDLog        []TDLogEntry

	// Advanced Stats
	AdvancedPassing   []AdvancedPassingRow
	AdvancedRushing   []AdvancedRushingRow
	AdvancedReceiving []AdvancedReceivingRow
	AdvancedDefense   []AdvancedDefenseRow

	// Extra tables (detected dynamically from JSON)
	ExtraTables []NormalizedTable

	// Internationalization
	Legends map[string]string // Flattened table legends for header translation
}

type TeamPassingStats struct {
	Cmp   int
	Att   int
	Pct   float64
	Yds   int
	YPA   float64
	TD    int
	Int   int
	Sacks int
}

type TeamRushingStats struct {
	Att  int
	Yds  int
	YPA  float64
	YPG  float64
	Long int
	TD   int
}

type TeamReceivingStats struct {
	Rec  int
	Yds  int
	YPR  float64
	YPG  float64
	Long int
	TD   int
}

type TeamDefenseStats struct {
	Int     int
	IntYds  int
	IntTD   int
	Sacks   float64
	SackYds int
	Tackles int
	Assists int
	Total   int
}

type TeamSpecialTeamsStats struct {
	KickRetYds  int
	KickRetAvg  float64
	KickRetLong int
	KickRetTD   int

	PuntRetYds  int
	PuntRetAvg  float64
	PuntRetLong int
	PuntRetTD   int

	PuntYds     int
	PuntAvg     float64
	PuntLong    int
	PuntBlocked int
	Touchbacks  int
	In20        int
	NetAvg      float64

	OutOfBounds int
	FairCatches int
	Downs       int
}

type ScoringTotals struct {
	Touchdowns     int
	RushTD         int
	PassTD         int
	KickRetTD      int
	PuntRetTD      int
	IntRetTD       int
	FieldGoalsMade int
	FieldGoalsAtt  int
	XPMade         int
	XPAtt          int
	TwoPtConv      int
	Safety         int
	OtherTD        int
	FumblesRecTD   int
}

type TeamScoringStats struct {
	Totals ScoringTotals
}

type PlayerPassingRow struct {
	Name  string
	Cmp   int
	Att   int
	Pct   float64
	Yds   int
	YPA   float64
	TD    int
	Int   int
	Sacks int
}

type PlayerRushingRow struct {
	Name string
	Att  int
	Yds  int
	YPA  float64
	YPG  float64
	Long int
	TD   int
}

type PlayerReceivingRow struct {
	Name string
	Rec  int
	Yds  int
	YPR  float64
	YPG  float64
	Long int
	TD   int
}

type PlayerDefenseRow struct {
	Name    string
	Int     int
	IntYds  int
	IntTD   int
	Sacks   float64
	SackYds int
	Tackles int
	Assists int
	Total   int
}

type PlayerScoringRow struct {
	Name      string
	TotalTD   int
	RushTD    int
	RecTD     int
	KickRetTD int
	PuntRetTD int
	IntRetTD  int
	FGM       int
	FGA       int
	XPM       int
	XPA       int
	TwoPt     int
	Safety    int
	Points    int
}

// Special Teams Player Tables
type KickingPlayer struct {
	Name    string
	FGM     int
	FGA     int
	FGPct   float64
	FGLong  int
	XPM     int
	XPA     int
	XPPct   float64
	KO      int
	KOYds   int
	KOTB    int
	KOTBPct float64
}

type PuntingPlayer struct {
	Name         string
	Punts        int
	Yds          int
	YdsPerPun    float64
	Long         int
	In20         int
	TB           int
	Blk          int
	NetYdsPerPun float64
}

type ReturnsPlayer struct {
	Name     string
	KRet     int
	KRetYds  int
	KRetAvg  float64
	KRetLong int
	KRetTD   int
	PRet     int
	PRetYds  int
	PRetAvg  float64
	PRetLong int
	PRetTD   int
}

// Team Conversions
type TeamConversions struct {
	ThirdDownSuccess  int
	ThirdDownAtt      int
	ThirdDownPct      float64
	FourthDownSuccess int
	FourthDownAtt     int
	FourthDownPct     float64
	RedZoneScores     int
	RedZoneAtt        int
	RedZonePct        float64
}

// TD Log Entry
type TDLogEntry struct {
	Player      string
	Quarter     string
	Time        string
	Distance    string
	Type        string // Rush, Pass, Return, etc.
	Description string
}

// Advanced Stats Tables
type AdvancedPassingRow struct {
	Name               string
	CmpAirYds          int
	CmpAirYdsPerAtt    float64
	CmpAirYdsPerCmp    float64
	Att                int
	BattedPasses       int
	Blitzed            int
	Cmp                int
	DropPct            float64
	Drops              int
	Hits               int
	Hurried            int
	OnTarget           int
	OnTargetPct        float64
	PlayAction         int
	PlayActionPassYds  int
	PoorThrowPct       float64
	PoorThrows         int
	Pressured          int
	PressuredPct       float64
	RPO                int
	RPOPassAtt         int
	RPOPassYds         int
	RPORushAtt         int
	RPORushYds         int
	RPOYds             int
	Spikes             int
	TargetYds          int
	TgtYdsPerAtt       float64
	Throwaways         int
	YAC                int
	YACPerCmp          float64
	PocketTime         float64
	Scrambles          int
	ScramblesYdsPerAtt float64
}

type AdvancedRushingRow struct {
	Name                     string
	RushAtt                  int
	RushBrokenTackles        int
	RushBrokenTacklesPerRush float64
	RushFirstDown            int
	RushYAC                  int
	RushYACPerRush           float64
	RushYds                  int
	RushYdsBeforeContact     int
	RushYdsBCPerRush         float64
}

type AdvancedReceivingRow struct {
	Name                   string
	Rec                    int
	RecADOT                float64
	RecAirYds              int
	RecAirYdsPerRec        float64
	RecBrokenTackles       int
	RecBrokenTacklesPerRec float64
	RecDropPct             float64
	RecDrops               int
	RecFirstDown           int
	RecPassRating          float64
	RecTargetInt           int
	RecYAC                 int
	RecYACPerRec           float64
	RecYds                 int
	Targets                int
}

type AdvancedDefenseRow struct {
	Name             string
	Age              int
	Blitzes          int
	DefAirYds        int
	DefBattedPasses  int
	DefCmp           int
	DefCmpPct        float64
	DefCmpTD         int
	DefCmpYds        int
	DefInt           int
	DefPassRating    float64
	DefTargets       int
	DefTgtYdsPerAtt  float64
	DefYAC           int
	DefYdsPerCmp     float64
	DefYdsPerTarget  float64
	Games            int
	GamesStarted     int
	Pressures        int
	QBHurry          int
	QBKnockdown      int
	Sacks            float64
	TacklesCombined  int
	TacklesMissed    int
	TacklesMissedPct float64
}
