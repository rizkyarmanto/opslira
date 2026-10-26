package reporting

import (
	"math"
	"testing"
	"time"

	"opslira/backend/internal/model"
)

// DowntimeForWindow is SlaPeriodService::downtimeForWindow: DOWN seconds are
// the clipped/merged DOWN segments minus any maintenance overlap (maintenance
// wins), and SLA = (total - maintenance - downtime) / (total - maintenance).
func TestDowntimeForWindow(t *testing.T) {
	start := time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour) // 3600s window

	t.Run("no data yields nil SLA", func(t *testing.T) {
		core := DowntimeForWindow(start, end, nil, nil, false)
		if core.SLA != nil {
			t.Errorf("hasData=false should force nil SLA, got %v", *core.SLA)
		}
		if core.TotalSeconds != 3600 {
			t.Errorf("TotalSeconds = %d, want 3600", core.TotalSeconds)
		}
	})

	t.Run("single down segment", func(t *testing.T) {
		down := []Segment{{Start: start.Add(10 * time.Minute), End: start.Add(20 * time.Minute)}}
		core := DowntimeForWindow(start, end, nil, down, true)
		if core.DowntimeSeconds != 600 {
			t.Errorf("DowntimeSeconds = %d, want 600", core.DowntimeSeconds)
		}
		wantSLA := (3600.0 - 600.0) / 3600.0 * 100
		if core.SLA == nil || math.Abs(*core.SLA-wantSLA) > 1e-6 {
			t.Errorf("SLA = %v, want %v", core.SLA, wantSLA)
		}
	})

	t.Run("maintenance wins over overlapping down", func(t *testing.T) {
		window := []Segment{{Start: start.Add(10 * time.Minute), End: start.Add(20 * time.Minute)}}
		down := []Segment{{Start: start.Add(10 * time.Minute), End: start.Add(20 * time.Minute)}}
		core := DowntimeForWindow(start, end, window, down, true)
		if core.DowntimeSeconds != 0 {
			t.Errorf("DowntimeSeconds = %d, want 0 (maintenance absorbs the down window)", core.DowntimeSeconds)
		}
		if core.MaintenanceSeconds != 600 {
			t.Errorf("MaintenanceSeconds = %d, want 600", core.MaintenanceSeconds)
		}
		// denominator = 3600 - 600 = 3000, downtime 0 -> 100%.
		if core.SLA == nil || math.Abs(*core.SLA-100.0) > 1e-6 {
			t.Errorf("SLA = %v, want 100", core.SLA)
		}
	})

	t.Run("fully maintained window is nil not zero", func(t *testing.T) {
		window := []Segment{{Start: start, End: end}}
		core := DowntimeForWindow(start, end, window, nil, true)
		// denominator = 3600 - 3600 = 0 -> nil (distinct from 0%).
		if core.SLA != nil {
			t.Errorf("fully maintained window SLA = %v, want nil", *core.SLA)
		}
	})
}

// IsDown classifies a log row: DOWN status, HTTP 5xx, or latency above the
// target's degraded threshold.
func TestIsDown(t *testing.T) {
	target := model.Target{DegradedThresholdMs: 1000}

	if !IsDown("DOWN", nil, nil, target) {
		t.Error("status DOWN should be down")
	}
	http500 := 500
	if !IsDown("UP", &http500, nil, target) {
		t.Error("HTTP 500 should be down")
	}
	slow := 1500.0
	if !IsDown("UP", nil, &slow, target) {
		t.Error("latency above threshold should be down")
	}
	http200 := 200
	fast := 200.0
	if IsDown("UP", &http200, &fast, target) {
		t.Error("healthy UP/200/fast should not be down")
	}
}
