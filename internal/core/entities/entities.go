package entities

import "time"

type SequenceDayOffsets []int

var SequenceStandard = SequenceDayOffsets{0, 3, 5, 7}

func (s SequenceDayOffsets) Effective() SequenceDayOffsets {
	if len(s) == 0 {
		return SequenceStandard
	}
	return s
}

func (s SequenceDayOffsets) IndexOf(offset int) (int, bool) {
	for i, dayOffset := range s.Effective() {
		if dayOffset == offset {
			return i, true
		}
	}
	return 0, false
}

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

func (c Client) ReminderSchedule() ReminderSchedule {
	return ReminderSchedule{
		PeriodType: c.PeriodType,
		Region:     c.Region,
		Sequence:   c.Sequence,
	}
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

type HolidayChecker interface {
	IsHoliday(date time.Time, region ClientRegion) (bool, error)
}

type ReminderSchedule struct {
	PeriodType PeriodType
	Region     ClientRegion
	Sequence   SequenceDayOffsets
}

type SequenceMatch struct {
	Period            Period
	SequenceDayOffset int
	SequenceIndex     int
}

func (s ReminderSchedule) MatchAt(at time.Time, holidays HolidayChecker) (SequenceMatch, bool, error) {
	period := CurrentPeriod(s.PeriodType, at)

	offset, ok, err := period.SequenceDayOffsetAt(at, s.Region, holidays)
	if err != nil {
		return SequenceMatch{}, false, err
	}
	if !ok {
		return SequenceMatch{}, false, nil
	}

	index, ok := s.Sequence.Effective().IndexOf(offset)
	if !ok {
		return SequenceMatch{}, false, nil
	}

	return SequenceMatch{
		Period:            period,
		SequenceDayOffset: offset,
		SequenceIndex:     index,
	}, true, nil
}
