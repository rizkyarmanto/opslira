package runner

import (
	"testing"
	"time"
)

func TestNextDaily(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("load timezone: %v", err)
	}

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before the target hour stays on the same day",
			now:  time.Date(2026, 1, 10, 7, 0, 0, 0, loc),
			want: time.Date(2026, 1, 10, 8, 5, 0, 0, loc),
		},
		{
			name: "after the target hour rolls to tomorrow",
			now:  time.Date(2026, 1, 10, 9, 30, 0, 0, loc),
			want: time.Date(2026, 1, 11, 8, 5, 0, 0, loc),
		},
		{
			name: "exactly at the target hour rolls to tomorrow",
			now:  time.Date(2026, 1, 10, 8, 5, 0, 0, loc),
			want: time.Date(2026, 1, 11, 8, 5, 0, 0, loc),
		},
		{
			name: "just before midnight keeps the day boundary",
			now:  time.Date(2026, 1, 10, 23, 59, 0, 0, loc),
			want: time.Date(2026, 1, 11, 8, 5, 0, 0, loc),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := nextDaily(test.now, "08:05", loc)
			if !got.Equal(test.want) {
				t.Errorf("nextDaily(%s) = %s, want %s", test.now, got, test.want)
			}
			if !got.After(test.now) {
				t.Errorf("nextDaily(%s) = %s, which is not after now", test.now, got)
			}
		})
	}
}
