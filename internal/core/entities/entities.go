package entities

import "time"

type MinimumBusinessDayGaps []int

var ReminderGapsStandard = MinimumBusinessDayGaps{0, 3, 2, 2}

func (g MinimumBusinessDayGaps) Effective() MinimumBusinessDayGaps {
	if len(g) == 0 {
		return ReminderGapsStandard
	}
	return g
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
	ReminderGaps MinimumBusinessDayGaps
	Region       ClientRegion
	Email        string
	Greeting     string
	FolderURL    string
	UploadPrompt string
}

func (c Client) ReminderSchedule() ReminderSchedule {
	return ReminderSchedule{
		PeriodType:   c.PeriodType,
		Region:       c.Region,
		ReminderGaps: c.ReminderGaps,
	}
}

type SendLogEntry struct {
	ForPeriod     Period
	ReminderGaps  MinimumBusinessDayGaps
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
	CompletionVerdictNotRequested CompletionVerdict = iota
	CompletionUndecided
	CompletionIncomplete
	CompletionComplete
)

type HolidayChecker interface {
	IsHoliday(date time.Time, region ClientRegion) (bool, error)
}

type ReminderSchedule struct {
	PeriodType   PeriodType
	Region       ClientRegion
	ReminderGaps MinimumBusinessDayGaps
}

type ReminderEligibility struct {
	Period        Period
	SequenceIndex int
	EarliestDate  time.Time
}

func (s ReminderSchedule) NextEligibility(at time.Time, successfulSends []SendLogEntry, holidays HolidayChecker) (ReminderEligibility, bool, error) {
	period := CurrentPeriod(s.PeriodType, at)
	gaps := s.ReminderGaps.Effective()
	nextIndex := len(successfulSends)
	if nextIndex >= len(gaps) {
		return ReminderEligibility{}, false, nil
	}

	earliest, err := s.earliestDate(period, nextIndex, successfulSends, holidays)
	if err != nil {
		return ReminderEligibility{}, false, err
	}

	ok, err := s.CanSendOn(at, earliest, holidays)
	if err != nil {
		return ReminderEligibility{}, false, err
	}
	if !ok {
		return ReminderEligibility{}, false, nil
	}

	return ReminderEligibility{
		Period:        period,
		SequenceIndex: nextIndex,
		EarliestDate:  earliest,
	}, true, nil
}

func (s ReminderSchedule) earliestDate(period Period, sequenceIndex int, successfulSends []SendLogEntry, holidays HolidayChecker) (time.Time, error) {
	gaps := s.ReminderGaps.Effective()
	if sequenceIndex == 0 {
		firstDay, err := period.FirstSequenceDay(s.Region, holidays)
		if err != nil {
			return time.Time{}, err
		}
		return AddBusinessDays(firstDay, gaps[0], s.Region, holidays)
	}

	previousSentAt := successfulSends[sequenceIndex-1].SentAt
	return AddBusinessDays(previousSentAt, gaps[sequenceIndex], s.Region, holidays)
}

func (s ReminderSchedule) CanSendOn(at time.Time, earliest time.Time, holidays HolidayChecker) (bool, error) {
	currentDay := normalizeDate(at)
	earliestDay := normalizeDate(earliest)

	if currentDay.Before(earliestDay) {
		return false, nil
	}
	if !isBusinessWeekday(currentDay) {
		return false, nil
	}

	holiday, err := isHoliday(currentDay, s.Region, holidays)
	if err != nil {
		return false, err
	}
	return !holiday, nil
}
