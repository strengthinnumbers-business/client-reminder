# Business Entity Types

```go
type PeriodType int

const (
	PeriodWeekly PeriodType = iota
	PeriodMonthly
	PeriodQuarterly
)

// Suggested ID format
// - Weekly: ISO week key, e.g. `2026-W07`
// - Monthly: calendar month key, e.g. `2026-02`
// - Quarterly: calendar quarter key, e.g. `2026-Q1`
type Period struct {
	Type PeriodType
	ID   string
}

type SequenceDayOffsets []int

var SequenceStandard = SequenceDayOffsets{0, 3, 5, 7}

type ClientRegion string

const (
	RegionAlberta              ClientRegion = "AB"
	RegionBritishColumbia      ClientRegion = "BC"
	RegionManitoba             ClientRegion = "MB"
	RegionNewBrunswick         ClientRegion = "NB"
	RegionNewfoundlandLabrador ClientRegion = "NL"
	RegionNorthwestTerritories ClientRegion = "NT"
	RegionNovaScotia           ClientRegion = "NS"
	RegionNunavut              ClientRegion = "NU"
	RegionOntario              ClientRegion = "ON"
	RegionPrinceEdwardIsland   ClientRegion = "PE"
	RegionQuebec               ClientRegion = "QC"
	RegionSaskatchewan         ClientRegion = "SK"
	RegionYukon                ClientRegion = "YT"
)

type Client struct {
	ID           string
	Name         string
	PeriodType   PeriodType
	Sequence     SequenceDayOffsets
	Region       ClientRegion
	Email        string
	Greeting     string
	FolderURL    string
	UploadPrompt string
}

type SendLogEntry struct {
	ForPeriod     Period
	Sequence      SequenceDayOffsets
	SequenceIndex int
	SentAt        time.Time
	Success       bool
	ErrorMessage  string
}

type ClientState struct {
	ClientID string
	SendLog  []SendLogEntry
}

type CompletionVerdict int

const (
	CompletionUndecided CompletionVerdict = iota
	CompletionIncomplete
	CompletionComplete
)
```
