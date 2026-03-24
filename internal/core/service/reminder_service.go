package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type Clock func() time.Time

type ReminderService struct {
	emailSender       ports.EmailSender
	clientRepo        ports.ClientRepository
	globalConfig      ports.GlobalConfiguration
	completionDecider ports.CompletionDecider
	holidayChecker    ports.HolidayChecker
	clock             Clock
}

type RunResult struct {
	TotalCustomers int
	SkippedDone    int
	Sent           int
	Failures       int
}

func NewReminderService(
	emailSender ports.EmailSender,
	clientRepo ports.ClientRepository,
	globalConfig ports.GlobalConfiguration,
	completionDecider ports.CompletionDecider,
	holidayChecker ports.HolidayChecker,
	clock Clock,
) *ReminderService {
	if clock == nil {
		clock = time.Now
	}

	return &ReminderService{
		emailSender:       emailSender,
		clientRepo:        clientRepo,
		globalConfig:      globalConfig,
		completionDecider: completionDecider,
		holidayChecker:    holidayChecker,
		clock:             clock,
	}
}

func (s *ReminderService) Run(ctx context.Context) (RunResult, error) {
	_ = ctx

	clients, err := s.clientRepo.GetAllClients()
	if err != nil {
		return RunResult{}, fmt.Errorf("load clients: %w", err)
	}

	template, err := s.globalConfig.GetEmailBodyTemplate()
	if err != nil {
		return RunResult{}, fmt.Errorf("load email template: %w", err)
	}

	now := s.clock().UTC()
	result := RunResult{TotalCustomers: len(clients)}

	for _, client := range clients {
		sequenceDayOffset, ok, err := s.calculateSequenceDayOffset(client.PeriodType, client.Region, now)
		if err != nil {
			result.Failures++
			continue
		}
		if !ok {
			continue
		}

		sequence := effectiveSequence(client.Sequence)
		sequenceIndex := indexOfSequenceDayOffset(sequence, sequenceDayOffset)
		if sequenceIndex < 0 {
			continue
		}

		period := entities.CurrentPeriod(client.PeriodType, now)
		log.Printf(
			"client %s sequence match: period=%s sequence_day_offset=%d sequence_index=%d",
			client.ID,
			period.ID,
			sequenceDayOffset,
			sequenceIndex,
		)

		verdict, err := s.completionDecider.IsCompleted(client, period)
		if err != nil {
			result.Failures++
			continue
		}

		if verdict == entities.CompletionComplete {
			result.SkippedDone++
			continue
		}

		if verdict == entities.CompletionUndecided || verdict == entities.CompletionIncomplete {
			body := RenderEmailTemplate(template, client, period, now)
			if err := s.emailSender.SendEmail(client.Email, body); err != nil {
				result.Failures++
				continue
			}
			result.Sent++
			if err := s.completionDecider.ResetCompletionVerdict(client, period); err != nil {
				result.Failures++
			}
		}

	}

	return result, nil
}

func (s *ReminderService) calculateSequenceDayOffset(periodType entities.PeriodType, region entities.ClientRegion, at time.Time) (int, bool, error) {
	currentDay := normalizeDate(at)
	if !isBusinessWeekday(currentDay) {
		return 0, false, nil
	}

	isHoliday, err := s.isHoliday(currentDay, region)
	if err != nil {
		return 0, false, err
	}
	if isHoliday {
		return 0, false, nil
	}

	periodStart := entities.CurrentPeriod(periodType, currentDay).Start()
	sequenceStart, err := s.firstSequenceDay(periodStart, region)
	if err != nil {
		return 0, false, err
	}
	if currentDay.Before(sequenceStart) {
		return 0, false, nil
	}

	offset := 0
	for day := sequenceStart; day.Before(currentDay); day = day.AddDate(0, 0, 1) {
		if !isBusinessWeekday(day) {
			continue
		}

		isHoliday, err := s.isHoliday(day, region)
		if err != nil {
			return 0, false, err
		}
		if isHoliday {
			continue
		}

		offset++
	}

	return offset, true, nil
}

func (s *ReminderService) firstSequenceDay(periodStart time.Time, region entities.ClientRegion) (time.Time, error) {
	day := periodStart
	for day.Weekday() != time.Monday {
		day = day.AddDate(0, 0, 1)
	}

	for {
		if !isBusinessWeekday(day) {
			day = day.AddDate(0, 0, 1)
			continue
		}

		isHoliday, err := s.isHoliday(day, region)
		if err != nil {
			return time.Time{}, err
		}
		if !isHoliday {
			return day, nil
		}

		day = day.AddDate(0, 0, 1)
	}
}

func (s *ReminderService) isHoliday(day time.Time, region entities.ClientRegion) (bool, error) {
	if s.holidayChecker == nil {
		return false, nil
	}
	return s.holidayChecker.IsHoliday(day, region)
}

func normalizeDate(at time.Time) time.Time {
	year, month, day := at.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func isBusinessWeekday(day time.Time) bool {
	return day.Weekday() >= time.Monday && day.Weekday() <= time.Friday
}

func indexOfSequenceDayOffset(sequence entities.SequenceDayOffsets, offset int) int {
	for i, dayOffset := range sequence {
		if dayOffset == offset {
			return i
		}
	}
	return -1
}

func effectiveSequence(sequence entities.SequenceDayOffsets) entities.SequenceDayOffsets {
	if len(sequence) == 0 {
		return entities.SequenceStandard
	}
	return sequence
}

func RenderEmailTemplate(template string, client entities.Client, period entities.Period, now time.Time) string {
	replacer := strings.NewReplacer(
		"{{ClientName}}", client.Name,
		"{{Greeting}}", client.Greeting,
		"{{FolderURL}}", client.FolderURL,
		"{{UploadPrompt}}", client.UploadPrompt,
		"{{PeriodID}}", period.ID,
		"{{RunDate}}", now.Format("2006-01-02"),
	)
	return replacer.Replace(template)
}
