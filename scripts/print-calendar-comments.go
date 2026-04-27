// Prints pasteable Go-comment calendars, to help with reasoning about the scheduling tests.

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

type intFlag struct {
	value int
	set   bool
}

func (f *intFlag) String() string {
	return fmt.Sprintf("%d", f.value)
}

func (f *intFlag) Set(value string) error {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("must be an integer")
	}
	if parsed < 0 {
		return fmt.Errorf("must be greater than or equal to 0")
	}
	f.value = parsed
	f.set = true
	return nil
}

func main() {
	if err := run(os.Stdout, os.Args[1:], time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "print-calendar-comments: %v\n", err)
		os.Exit(2)
	}
}

func run(out io.Writer, args []string, now time.Time) error {
	flags := flag.NewFlagSet("print-calendar-comments", flag.ContinueOnError)

	currentYear := now.Year()
	year := flags.Int("year", currentYear, "year to print")
	indentTabs := &intFlag{value: 1}
	indentSpaces := &intFlag{value: 4}
	flags.Var(indentTabs, "indent-tabs", "number of leading tabs")
	flags.Var(indentSpaces, "indent-spaces", "number of leading spaces")
	commentStyle := flags.String("comment-style", "inline", "comment style: inline or block")
	padding := flags.Int("padding", 1, "spaces around each day number and weekday abbreviation")
	weekendLine := flags.Bool("weekend-line", false, "draw a line with pipe symbols between Friday and Saturday")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *year < 1 {
		return fmt.Errorf("--year must be greater than 0")
	}
	if indentTabs.set && indentSpaces.set {
		return fmt.Errorf("--indent-tabs and --indent-spaces are mutually exclusive")
	}
	if *commentStyle != "inline" && *commentStyle != "block" {
		return fmt.Errorf("--comment-style must be inline or block")
	}
	if *padding < 0 {
		return fmt.Errorf("--padding must be greater than or equal to 0")
	}

	indent := strings.Repeat("\t", indentTabs.value)
	if indentSpaces.set {
		indent = strings.Repeat(" ", indentSpaces.value)
	}

	for month := time.January; month <= time.December; month++ {
		if month > time.January {
			fmt.Fprintln(out)
		}
		printMonth(out, *year, month, indent, *commentStyle, *padding, *weekendLine)
	}

	return nil
}

func printMonth(out io.Writer, year int, month time.Month, indent, commentStyle string, padding int, weekendLine bool) {
	lines := calendarMonthLines(year, month, padding, weekendLine)
	switch commentStyle {
	case "block":
		fmt.Fprintf(out, "%s/*\n", indent)
		for _, line := range lines {
			fmt.Fprintf(out, "%s%s\n", indent, line)
		}
		fmt.Fprintf(out, "%s*/\n", indent)
	default:
		for _, line := range lines {
			commentSpacer := " "
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "-") {
				commentSpacer = ""
			}
			fmt.Fprintf(out, "%s//%s%s\n", indent, commentSpacer, line)
		}
	}
}

func calendarMonthLines(year int, month time.Month, padding int, weekendLine bool) []string {
	lines := []string{
		fmt.Sprintf("%s %d", month.String(), year),
		weekdayHeader(padding, weekendLine),
		separatorLine(padding, weekendLine),
	}

	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
	weekStart := firstOfMonth.AddDate(0, 0, -daysSinceMonday(firstOfMonth))

	for day := weekStart; !day.After(lastOfMonth); day = day.AddDate(0, 0, 7) {
		lines = append(lines, weekLine(day, month, padding, weekendLine))
	}

	lines = append(lines, separatorLine(padding, weekendLine))
	return lines
}

func weekdayHeader(padding int, weekendLine bool) string {
	return strings.Join(dayCells([]string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "So"}, padding, weekendLine), "")
}

func weekLine(weekStart time.Time, month time.Month, padding int, weekendLine bool) string {
	values := make([]string, 0, 7)
	for offset := 0; offset < 7; offset++ {
		day := weekStart.AddDate(0, 0, offset)
		if day.Month() != month {
			values = append(values, "")
			continue
		}
		values = append(values, fmt.Sprintf("%02d", day.Day()))
	}
	return strings.Join(dayCells(values, padding, weekendLine), "")
}

func dayCells(values []string, padding int, weekendLine bool) []string {
	cells := make([]string, 0, len(values)+1)
	cellPadding := strings.Repeat(" ", padding)
	for index, value := range values {
		if weekendLine && index == 5 {
			cells = append(cells, "|")
		}
		cells = append(cells, cellPadding+fmt.Sprintf("%-2s", value)+cellPadding)
	}
	return cells
}

func separatorLine(padding int, weekendLine bool) string {
	weekdayWidth := 5 * (2 + 2*padding)
	weekendWidth := 2 * (2 + 2*padding)
	if !weekendLine {
		return strings.Repeat("-", weekdayWidth+weekendWidth)
	}
	return strings.Repeat("-", weekdayWidth) + "|" + strings.Repeat("-", weekendWidth)
}

func daysSinceMonday(day time.Time) int {
	weekday := int(day.Weekday())
	if weekday == 0 {
		return 6
	}
	return weekday - 1
}
