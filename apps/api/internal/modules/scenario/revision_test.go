package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func manualRevision() UpdateRequest {
	return UpdateRequest{
		Name:        "人工订单场景",
		Description: "人工修订",
		Status:      "active",
		Steps: []UpdateStep{
			{Key: "login", Name: "登录", Type: "http", RequestConfig: json.RawMessage(`{"method":"POST","path":"/login"}`)},
			{Key: "wait", Name: "等待", Type: "delay", DependsOn: []string{"login"}, RequestConfig: json.RawMessage(`{"milliseconds":100}`)},
		},
	}
}

func TestMemoryUpdateCreatesImmutableManualVersion(t *testing.T) {
	repo := NewMemoryRepository([]PromotionCandidate{acceptedCandidate()})
	created, err := repo.Promote(context.Background(), systemA, discoveryID, candidateID, userID)
	if err != nil {
		t.Fatal(err)
	}
	old := created.Detail
	updated, err := repo.Update(context.Background(), systemA, old.Scenario.ID, userID, manualRevision())
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version.VersionNo != old.Version.VersionNo+1 || updated.Version.SourceType != "manual" {
		t.Fatalf("version=%#v", updated.Version)
	}
	if updated.Version.ID == old.Version.ID || updated.Steps[0].ID == old.Steps[0].ID {
		t.Fatal("revision reused immutable version or step UUID")
	}
	if updated.Scenario.CurrentVersionID != updated.Version.ID || updated.Scenario.Name != "人工订单场景" || updated.Scenario.Status != "active" {
		t.Fatalf("scenario=%#v", updated.Scenario)
	}
	if updated.Steps[0].Position != 1 || updated.Steps[1].Position != 2 {
		t.Fatalf("positions=%#v", updated.Steps)
	}
	if got, found, err := repo.Get(context.Background(), systemA, old.Scenario.ID); err != nil || !found || got.Version.ID != updated.Version.ID {
		t.Fatalf("get found=%v err=%v detail=%#v", found, err, got)
	}
	if old.Version.VersionNo != 1 || old.Steps[0].ID == "" {
		t.Fatalf("old detail mutated=%#v", old)
	}
}

func TestUpdateValidationRejectsInvalidGraphsAndFields(t *testing.T) {
	cases := map[string]UpdateRequest{
		"blank name":         {Name: " ", Status: "draft"},
		"bad status":         {Name: "n", Status: "deleted"},
		"bad type":           {Name: "n", Status: "draft", Steps: []UpdateStep{{Key: "a", Name: "a", Type: "grpc"}}},
		"duplicate key":      {Name: "n", Status: "draft", Steps: []UpdateStep{{Key: "a", Name: "a", Type: "http"}, {Key: "a", Name: "b", Type: "http"}}},
		"missing dependency": {Name: "n", Status: "draft", Steps: []UpdateStep{{Key: "a", Name: "a", Type: "http", DependsOn: []string{"missing"}}}},
		"self dependency":    {Name: "n", Status: "draft", Steps: []UpdateStep{{Key: "a", Name: "a", Type: "http", DependsOn: []string{"a"}}}},
		"cycle": {Name: "n", Status: "draft", Steps: []UpdateStep{
			{Key: "a", Name: "a", Type: "http", DependsOn: []string{"b"}},
			{Key: "b", Name: "b", Type: "script", DependsOn: []string{"a"}},
		}},
		"invalid request config": {Name: "n", Status: "draft", Steps: []UpdateStep{{Key: "a", Name: "a", Type: "http", RequestConfig: json.RawMessage(`[]`)}}},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ValidateUpdate(input); !errors.Is(err, ErrInvalidRevision) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestMemoryUpdateScopesScenarioBySystem(t *testing.T) {
	repo := NewMemoryRepository([]PromotionCandidate{acceptedCandidate()})
	created, err := repo.Promote(context.Background(), systemA, discoveryID, candidateID, userID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Update(context.Background(), systemB, created.Scenario.ID, userID, manualRevision())
	if !errors.Is(err, ErrScenarioNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestStepRequestConfigSerializesAsJSONObjectAndOmitsAbsentValue(t *testing.T) {
	payload, err := json.Marshal([]Step{
		{Key: "configured", RequestConfig: json.RawMessage(`{"method":"GET","path":"/orders"}`)},
		{Key: "absent"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	config, ok := decoded[0]["requestConfig"].(map[string]any)
	if !ok || config["method"] != "GET" || config["path"] != "/orders" {
		t.Fatalf("configured requestConfig must be an object: %s", payload)
	}
	if _, exists := decoded[1]["requestConfig"]; exists {
		t.Fatalf("absent requestConfig must be omitted: %s", payload)
	}
}
