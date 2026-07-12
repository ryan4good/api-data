package importer

import (
	"encoding/json"
	"testing"
)

func TestImportNativeScenarioBundle(t *testing.T) {
	input := []byte(`{
      "schemaVersion":"1.0",
      "exportedAt":"2026-07-11T08:00:00Z",
      "system":{"key":"oms","name":"Order Management"},
      "scenario":{"key":"create-order","name":"Create order","version":1,
        "variables":[{"key":"token","scope":"environment","secretRef":"vault://oms/token","sensitive":true}],
        "steps":[{"key":"create","name":"Create","type":"http","dependsOn":[],"timeoutMs":30000,"failurePolicy":"stop",
          "apiOperationRef":{"operationKey":"POST /orders"},
          "request":{"connectorRef":"oms-http","method":"POST","url":"/orders","headers":{"Authorization":"Bearer {{token}}"}},
          "extractors":[{"key":"orderId","source":"json_body","expression":"$.data.id","required":true}]},
        {"key":"get","name":"Get","type":"http","dependsOn":["create"],"timeoutMs":30000,"failurePolicy":"stop",
          "request":{"connectorRef":"oms-http","method":"GET","url":"/orders/{{orderId}}"}}]}}
    `)

	result, err := Import(input, Options{})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.SourceFormat != FormatScenarioBundle {
		t.Fatalf("source format = %q", result.SourceFormat)
	}
	if result.Bundle.Scenario.Key != "create-order" || len(result.Bundle.Scenario.Steps) != 2 {
		t.Fatalf("unexpected bundle: %#v", result.Bundle)
	}
	if result.Bundle.Scenario.Steps[0].APIOperationRef == nil || len(result.Bundle.Scenario.Steps[0].Extractors) != 1 {
		t.Fatalf("lossy native import: %#v", result.Bundle.Scenario.Steps[0])
	}
	if result.Report.Counts.Requests != 2 || result.Report.Counts.Variables != 1 {
		t.Fatalf("unexpected counts: %#v", result.Report.Counts)
	}
	if len(result.Report.Errors) != 0 || len(result.Report.Warnings) != 0 {
		t.Fatalf("unexpected report: %#v", result.Report)
	}
}

func TestImportPostmanCollectionFlattensFoldersRedactsSecretsAndFlagsScripts(t *testing.T) {
	input := []byte(`{
      "info":{"name":"Order smoke","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
      "variable":[
        {"key":"baseUrl","value":"https://api.example.test"},
        {"key":"apiToken","value":"plain-secret"}
      ],
      "event":[{"listen":"prerequest","script":{"exec":["pm.variables.set('stamp', Date.now());"]}}],
      "item":[
        {"name":"Health","request":{"method":"GET","url":{"raw":"{{baseUrl}}/health"}}},
        {"name":"Orders","item":[
          {"name":"Approve order","request":{"method":"POST","header":[{"key":"Authorization","value":"Bearer {{apiToken}}"}],"url":{"raw":"{{baseUrl}}/orders/{{orderId}}/approve"},"body":{"mode":"raw","raw":"{\"approved\":true}","options":{"raw":{"language":"json"}}}},
           "event":[{"listen":"test","script":{"exec":["pm.response.to.have.status(200);"]}}]}
        ]}
      ]
    }`)

	result, err := Import(input, Options{SystemKey: "oms", SystemName: "Order Management", ConnectorRef: "oms-http"})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.SourceFormat != FormatPostmanCollection21 {
		t.Fatalf("source format = %q", result.SourceFormat)
	}
	if result.Bundle.System.Key != "oms" || result.Bundle.Scenario.Name != "Order smoke" {
		t.Fatalf("unexpected metadata: %#v", result.Bundle)
	}
	if result.Report.Counts.Requests != 2 || result.Report.Counts.Scripts != 2 || result.Report.Counts.Variables != 2 {
		t.Fatalf("unexpected counts: %#v", result.Report.Counts)
	}
	if result.Report.Counts.Assertions != 1 {
		t.Fatalf("recognized Postman test was not converted: %#v", result.Report.Counts)
	}

	var scriptSteps, requestSteps, assertions int
	for _, step := range result.Bundle.Scenario.Steps {
		if step.Type == "script" {
			scriptSteps++
			if step.Script == nil || !step.Script.RequiresReview {
				t.Fatalf("imported script must require review: %#v", step)
			}
		}
		if step.Type == "http" {
			requestSteps++
			assertions += len(step.Assertions)
		}
	}
	if scriptSteps != 2 || requestSteps != 2 {
		t.Fatalf("unexpected steps: %#v", result.Bundle.Scenario.Steps)
	}
	if assertions != 1 {
		t.Fatalf("expected one status assertion, steps: %#v", result.Bundle.Scenario.Steps)
	}

	var token Variable
	for _, variable := range result.Bundle.Scenario.Variables {
		if variable.Key == "apiToken" {
			token = variable
		}
	}
	if !token.Sensitive || token.Value != nil || token.SecretRef == "" {
		t.Fatalf("secret was not redacted: %#v", token)
	}
	assertIssueCode(t, result.Report.Warnings, "SECRET_VALUE_REDACTED")
	assertIssueCode(t, result.Report.Warnings, "UNBOUND_VARIABLE")

	encoded, err := json.Marshal(result.Bundle)
	if err != nil {
		t.Fatalf("bundle is not JSON serializable: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("empty encoded bundle")
	}
}

func TestImportRejectsUnsupportedDocument(t *testing.T) {
	result, err := Import([]byte(`{"hello":"world"}`), Options{})
	if err == nil {
		t.Fatal("Import() expected error")
	}
	assertIssueCode(t, result.Report.Errors, "UNSUPPORTED_FORMAT")
}

func TestImportRejectsInvalidJSON(t *testing.T) {
	result, err := Import([]byte(`{"info":`), Options{})
	if err == nil {
		t.Fatal("Import() expected error")
	}
	assertIssueCode(t, result.Report.Errors, "INVALID_JSON")
}

func assertIssueCode(t *testing.T, issues []Issue, code string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Code == code {
			return
		}
	}
	t.Fatalf("issue %q not found in %#v", code, issues)
}
