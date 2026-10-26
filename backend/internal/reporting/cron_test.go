package reporting

import (
	"testing"
	"time"
)

func TestValidCron(t *testing.T) {
	valid := []string{
		"0 8 * * *",
		"30 2 * * *",
		"0 8 * * 1",
		"0 8 1 * *",
		"*/15 * * * *",
		"0 0,12 1,15 * *",
		"5 4 * * 0,6",
		"0-30/5 * * * *",
		"15 4 * * 7",
	}
	for _, expr := range valid {
		if err := ValidCron(expr); err != nil {
			t.Errorf("ValidCron(%q) = %v, want nil", expr, err)
		}
	}

	invalid := []string{
		"",
		"not a cron",
		"* * * *",
		"* * * * * *",
		"60 * * * *",
		"* 24 * * *",
		"* * 32 * *",
		"* * * 13 *",
		"* * * * 8",
		"a b c d e",
		"*/0 * * * *",
	}
	for _, expr := range invalid {
		if err := ValidCron(expr); err == nil {
			t.Errorf("ValidCron(%q) = nil, want error %q", expr, cronInvalidMessage)
		} else if err.Error() != cronInvalidMessage {
			t.Errorf("ValidCron(%q) message = %q", expr, err)
		}
	}
}

func TestNextCronRun(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 7, 59, 30, 0, jakarta) // Sunday

	next, err := NextCronRun("0 8 * * *", "Asia/Jakarta", now)
	if err != nil {
		t.Fatalf("NextCronRun: %v", err)
	}
	want := time.Date(2026, 10, 25, 8, 0, 0, 0, jakarta)
	if !next.Equal(want) {
		t.Errorf("daily next = %s, want %s", next, want)
	}

	// Strictly after now: at exactly 08:00:00 the next run is tomorrow.
	atRun := time.Date(2026, 10, 26, 8, 0, 0, 0, jakarta)
	next, err = NextCronRun("0 8 * * *", "Asia/Jakarta", atRun)
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2026, 10, 27, 8, 0, 0, 0, jakarta)
	if !next.Equal(want) {
		t.Errorf("next after exact match = %s, want %s", next, want)
	}

	// Weekly (Monday) from Sunday 07:59 -> next day (Monday) 08:00.
	next, err = NextCronRun("0 8 * * 1", "Asia/Jakarta", now)
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2026, 10, 26, 8, 0, 0, 0, jakarta)
	if !next.Equal(want) {
		t.Errorf("weekly next = %s, want %s", next, want)
	}

	// Monthly (1st) from Oct 25 -> Nov 1 08:00.
	next, err = NextCronRun("0 8 1 * *", "Asia/Jakarta", now)
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2026, 11, 1, 8, 0, 0, 0, jakarta)
	if !next.Equal(want) {
		t.Errorf("monthly next = %s, want %s", next, want)
	}
}

// TestNextCronRunDayOfWeekOrRule mirrors standard cron: when both day fields
// are restricted, either may match (mtdownton/cron-expression behavior).
func TestNextCronRunDayOfWeekOrRule(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	// Oct 26 2026 is a Monday... anchor to a Wednesday and match the 1st too.
	now := time.Date(2026, 10, 28, 9, 0, 0, 0, jakarta) // Wednesday

	// "0 8 1 * 3": next run matching day-of-month=1 OR Wednesday.
	next, err := NextCronRun("0 8 1 * 3", "Asia/Jakarta", now)
	if err != nil {
		t.Fatal(err)
	}
	// The next Wednesday 08:00 (Nov 4) vs Nov 1 (Sunday): Wednesday Nov 4
	// is later than Nov 1, so the OR rule yields Nov 1 08:00 first.
	want := time.Date(2026, 11, 1, 8, 0, 0, 0, jakarta)
	if !next.Equal(want) {
		t.Errorf("dom-or-dow next = %s, want %s", next, want)
	}
}

func TestNextCronRunTimezone(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 26, 7, 59, 0, 0, jakarta)

	// A different zone shifts the wall clock the cron matches on.
	next, err := NextCronRun("0 8 * * *", "UTC", now)
	if err != nil {
		t.Fatal(err)
	}
	if next.Location().String() != "UTC" {
		t.Errorf("next location = %s, want UTC", next.Location())
	}
	// 07:59 WIB = 07:59+07 -> 00:59 UTC; next 08:00 UTC is 7 hours later.
	if next.UTC().Hour() != 8 || next.UTC().Minute() != 0 {
		t.Errorf("next UTC = %s, want 08:00 UTC", next.UTC())
	}

	if _, err := NextCronRun("0 8 * * *", "Not/AZone", now); err == nil {
		t.Error("invalid timezone accepted")
	}
}
