package dashboard

import (
	"testing"

	"opslira/backend/internal/model"
)

func target(id, group, environment string) model.Target {
	return model.Target{ID: id, Name: "t-" + id, DashboardGroup: group, Environment: environment}
}

func TestFilterByGroupAndEnvironment(t *testing.T) {
	targets := []model.Target{
		target("1", "NETWORK", "INTERNAL"),
		target("2", "NETWORK", "PRODUCTION"),
		target("3", "APP_DOMAIN_DB", "PRODUCTION"),
	}

	network := filterByGroup(targets, "NETWORK")
	if len(network) != 2 || network[0].ID != "1" {
		t.Errorf("filterByGroup = %#v", network)
	}

	prod := filterByEnvironment(targets, "PRODUCTION")
	if len(prod) != 2 {
		t.Errorf("filterByEnvironment = %#v", prod)
	}
	if got := filterByEnvironment(targets, "STAGING"); len(got) != 0 {
		t.Errorf("unexpected STAGING matches: %#v", got)
	}
}

func TestTrafficSummaryReadsStateMetrics(t *testing.T) {
	targets := []model.Target{
		{
			Name: "router-a",
			State: &model.TargetState{
				SummaryMetrics: map[string]any{"rx_mbps": 12.5, "tx_mbps": 3},
			},
		},
		{Name: "router-b"}, // no state yet
	}

	summary := trafficSummary(targets)
	if summary["router-a"]["rx_mbps"] != 12.5 || summary["router-a"]["tx_mbps"] != 3 {
		t.Errorf("router-a traffic = %#v", summary["router-a"])
	}
	if summary["router-b"]["rx_mbps"] != 0 || summary["router-b"]["tx_mbps"] != 0 {
		t.Errorf("router-b traffic = %#v", summary["router-b"])
	}
}

func TestAsFloat(t *testing.T) {
	if got := asFloat(float64(1.5)); got != 1.5 {
		t.Errorf("asFloat(float64) = %v", got)
	}
	if got := asFloat(int(7)); got != 7 {
		t.Errorf("asFloat(int) = %v", got)
	}
	if got := asFloat("junk"); got != 0 {
		t.Errorf("asFloat(string) = %v, want 0", got)
	}
	if got := asFloat(nil); got != 0 {
		t.Errorf("asFloat(nil) = %v, want 0", got)
	}
}
