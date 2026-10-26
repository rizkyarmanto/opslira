package reporting

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Cron validation error messages (exact Laravel strings).
const (
	cronInvalidMessage = "The cron expression is not valid. Use five fields, for example 0 8 * * *."
)

// cronField is one parsed cron field: an explicit value set or "any".
type cronField struct {
	values map[int]bool
	any    bool
}

func (f cronField) matches(v int) bool {
	return f.any || f.values[v]
}

type cronSchedule struct {
	minute cronField // 0-59
	hour   cronField // 0-23
	dom    cronField // 1-31
	month  cronField // 1-12
	dow    cronField // 0-6 (7 normalized to 0)
	// domAny/dowAny drive the standard OR rule when both day fields are
	// restricted (classic cron: Jan 1 * * matches either constraint).
	domAny bool
	dowAny bool
}

// parseCron parses a standard five-field cron expression with lists, ranges
// and steps — the subset mtdownton/cron-expression accepts for
// CronExpression::isValidExpression.
func parseCron(expr string) (*cronSchedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf(cronInvalidMessage)
	}

	minute, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, err
	}
	hour, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, err
	}
	dom, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, err
	}
	month, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, err
	}
	// Day-of-week accepts 0-7 where 7 means Sunday.
	dow, err := parseField(fields[4], 0, 7)
	if err != nil {
		return nil, err
	}

	schedule := &cronSchedule{
		minute: minute,
		hour:   hour,
		dom:    dom,
		month:  month,
		dow:    dow,
		domAny: dom.any,
		dowAny: dow.any,
	}
	// Normalize 7 -> 0 (both mean Sunday).
	if dow.values[7] {
		delete(dow.values, 7)
		dow.values[0] = true
	}

	return schedule, nil
}

// ValidCron reports whether expr is a parseable five-field expression, using
// the Laravel validation message on failure.
func ValidCron(expr string) error {
	_, err := parseCron(expr)

	return err
}

// NextCronRun returns the first time strictly after now that expr matches in
// the given timezone (CronNextRunCalculator port).
func NextCronRun(expr, timezone string, now time.Time) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("timezone %q: %w", timezone, err)
	}
	schedule, err := parseCron(expr)
	if err != nil {
		return time.Time{}, err
	}

	next := schedule.next(now.In(loc))
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("cron expression %q has no run time within five years", expr)
	}

	return next, nil
}

func parseField(raw string, min, max int) (cronField, error) {
	field := cronField{values: map[int]bool{}}
	if raw == "*" {
		field.any = true

		return field, nil
	}

	for _, part := range strings.Split(raw, ",") {
		if part == "" {
			return field, fmt.Errorf(cronInvalidMessage)
		}

		step := 1
		rangePart := part
		if idx := strings.Index(part, "/"); idx >= 0 {
			rangePart = part[:idx]
			stepPart := part[idx+1:]
			step, err := strconv.Atoi(stepPart)
			if err != nil || step < 1 {
				return field, fmt.Errorf(cronInvalidMessage)
			}
		}

		var low, high int
		switch {
		case rangePart == "*":
			low, high = min, max
		case strings.Contains(rangePart, "-"):
			bounds := strings.SplitN(rangePart, "-", 2)
			var err error
			low, err = strconv.Atoi(bounds[0])
			if err != nil {
				return field, fmt.Errorf(cronInvalidMessage)
			}
			high, err = strconv.Atoi(bounds[1])
			if err != nil {
				return field, fmt.Errorf(cronInvalidMessage)
			}
		default:
			value, err := strconv.Atoi(rangePart)
			if err != nil {
				return field, fmt.Errorf(cronInvalidMessage)
			}
			low = value
			if strings.Contains(part, "/") {
				// "N/step" runs from N to the field maximum.
				high = max
			} else {
				high = value
			}
		}

		if low < min || high > max || low > high {
			return field, fmt.Errorf(cronInvalidMessage)
		}
		for v := low; v <= high; v += step {
			field.values[v] = true
		}
	}

	if len(field.values) == 0 {
		return field, fmt.Errorf(cronInvalidMessage)
	}

	return field, nil
}

// next finds the first matching instant strictly after now using structural
// search (month -> day -> hour -> minute) instead of brute-force minutes.
func (c *cronSchedule) next(now time.Time) time.Time {
	t := now.Add(time.Minute).Truncate(time.Minute)
	limit := now.AddDate(5, 0, 0)

	for t.Before(limit) {
		if !c.month.matches(int(t.Month())) {
			// Jump to the first instant of the next month.
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !c.dayMatches(t) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}
		if !c.hour.matches(t.Hour()) {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}
		if !c.minute.matches(t.Minute()) {
			t = t.Add(time.Minute)
			continue
		}

		return t
	}

	return time.Time{}
}

// dayMatches implements the standard cron day rule: when both day-of-month
// and day-of-week are restricted, either may match; otherwise both must.
func (c *cronSchedule) dayMatches(t time.Time) bool {
	domOK := c.dom.matches(t.Day())
	dowOK := c.dow.matches(int(t.Weekday()))

	switch {
	case !c.domAny && !c.dowAny:
		return domOK || dowOK
	case !c.domAny:
		return domOK
	case !c.dowAny:
		return dowOK
	default:
		return true
	}
}

// validTimezone reports whether tz loads (used by schedule validation).
func validTimezone(tz string) error {
	_, err := time.LoadLocation(tz)

	return err
}
