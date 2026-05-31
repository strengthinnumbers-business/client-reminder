package entities

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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
	EmailStyle   string
	Greeting     string
	FolderURL    string
	FolderPath   string
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
	ClientID      string
	ForPeriod     Period
	ReminderGaps  MinimumBusinessDayGaps
	SequenceIndex int
	SentAt        time.Time
	Success       bool
	ErrorMessage  string
}

type UploadSnapshot map[string]string

func (s UploadSnapshot) Filter(folderPath string) UploadSnapshot {
	filtered := UploadSnapshot{}
	if folderPath == "" {
		return filtered
	}

	folder := filepath.Clean(folderPath)
	for filePath, checksum := range s {
		cleanPath := filepath.Clean(filePath)
		if cleanPath == folder || strings.HasPrefix(cleanPath, folder+string(filepath.Separator)) {
			filtered[filePath] = checksum
		}
	}
	return filtered
}

type UploadChanges struct {
	Added   []string
	Changed []string
	Deleted []string
}

func DiffSnapshots(previous, current UploadSnapshot) UploadChanges {
	changes := UploadChanges{}
	for path, currentChecksum := range current {
		previousChecksum, ok := previous[path]
		if !ok {
			changes.Added = append(changes.Added, path)
			continue
		}
		if previousChecksum != currentChecksum {
			changes.Changed = append(changes.Changed, path)
		}
	}
	for path := range previous {
		if _, ok := current[path]; !ok {
			changes.Deleted = append(changes.Deleted, path)
		}
	}
	sort.Strings(changes.Added)
	sort.Strings(changes.Changed)
	sort.Strings(changes.Deleted)
	return changes
}

func (c UploadChanges) Any() bool {
	return len(c.Added) > 0 || len(c.Changed) > 0 || len(c.Deleted) > 0
}

func (c UploadChanges) Summary() string {
	var builder strings.Builder
	writeChangeList(&builder, "ADDED", c.Added)
	writeChangeList(&builder, "CHANGED", c.Changed)
	writeChangeList(&builder, "DELETED", c.Deleted)
	return strings.TrimSuffix(builder.String(), "\n")
}

func writeChangeList(builder *strings.Builder, heading string, paths []string) {
	if builder.Len() > 0 {
		builder.WriteString("\n")
	}
	builder.WriteString(heading)
	builder.WriteString("\n")
	for _, path := range paths {
		builder.WriteString(path)
		builder.WriteString("\n")
	}
}

type ClientState struct {
	ClientID string
	SendLog  []SendLogEntry
}

type CompletionVerdictStatus int

const (
	CompletionVerdictNotRequested CompletionVerdictStatus = iota
	CompletionUndecided
	CompletionIncomplete
	CompletionComplete
)

func (s CompletionVerdictStatus) Name() string {
	switch s {
	case CompletionVerdictNotRequested:
		return "not_requested"
	case CompletionUndecided:
		return "undecided"
	case CompletionIncomplete:
		return "upload_incomplete"
	case CompletionComplete:
		return "upload_complete"
	default:
		return fmt.Sprintf("unknown_%d", s)
	}
}

type CompletionVerdictTask struct {
	ID             string
	Status         CompletionVerdictStatus
	ChangesSummary string
	VerdictReason  string
}

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
