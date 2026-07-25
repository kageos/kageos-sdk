package app

import (
	"encoding/json"
	"testing"
)

func TestApiInfoIsEqualTreatsNilAndEmptyMetadataSlicesAsEqual(t *testing.T) {
	previous := &ApiInfo{
		Name: "orders",
		ConnectorEndpoints: []ConnectorEndpoint{{
			Provider:       "github",
			Method:         "GET",
			URL:            "/repos",
			RequiredScopes: nil,
		}},
		Schedules: nil,
	}
	current := &ApiInfo{
		Name: "orders",
		ConnectorEndpoints: []ConnectorEndpoint{{
			Provider:       "github",
			Method:         "GET",
			URL:            "/repos",
			RequiredScopes: []string{},
		}},
		Schedules: []CompiledFormSchedule{},
	}

	if !previous.IsEqual(current) {
		t.Fatal("expected nil and empty metadata slices to compare equal")
	}
}

func TestApiInfoIsEqualStillDetectsMetadataChanges(t *testing.T) {
	previous := &ApiInfo{
		Name: "orders",
		ConnectorEndpoints: []ConnectorEndpoint{{
			Provider: "github",
			Method:   "GET",
			URL:      "/repos",
		}},
	}
	current := &ApiInfo{
		Name: "orders",
		ConnectorEndpoints: []ConnectorEndpoint{{
			Provider: "github",
			Method:   "POST",
			URL:      "/repos",
		}},
	}

	if previous.IsEqual(current) {
		t.Fatal("expected a real connector endpoint change to be detected")
	}
}

func TestApiInfoIsEqualAfterSnapshotJSONRoundTrip(t *testing.T) {
	current := &ApiInfo{
		Name: "orders",
		ConnectorEndpoints: []ConnectorEndpoint{{
			Provider:       "github",
			Method:         "GET",
			URL:            "/repos",
			RequiredScopes: []string{},
		}},
		Schedules: []CompiledFormSchedule{},
	}
	data, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}

	var snapshot ApiInfo
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if !snapshot.IsEqual(current) {
		t.Fatal("expected JSON snapshot and unchanged live API to compare equal")
	}
}
