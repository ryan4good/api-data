package importer

import "encoding/json"

const (
	FormatScenarioBundle      = "scenario-bundle-1.0"
	FormatPostmanCollection21 = "postman-collection-2.1"
)

type Options struct {
	SystemKey    string
	SystemName   string
	ConnectorRef string
}

type Result struct {
	SourceFormat string       `json:"sourceFormat"`
	Bundle       Bundle       `json:"bundle"`
	Report       ImportReport `json:"report"`
}

type ImportReport struct {
	Counts   Counts  `json:"counts"`
	Warnings []Issue `json:"warnings"`
	Errors   []Issue `json:"errors"`
}

type Counts struct {
	Requests   int `json:"requests"`
	Scripts    int `json:"scripts"`
	Assertions int `json:"assertions"`
	Variables  int `json:"variables"`
}

type Issue struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	SourcePath string `json:"sourcePath,omitempty"`
}

type Bundle struct {
	SchemaVersion string         `json:"schemaVersion"`
	ExportedAt    string         `json:"exportedAt"`
	System        System         `json:"system"`
	Scenario      Scenario       `json:"scenario"`
	Extensions    map[string]any `json:"extensions,omitempty"`
}

type System struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type Scenario struct {
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Version     int            `json:"version"`
	Variables   []Variable     `json:"variables"`
	Steps       []Step         `json:"steps"`
	Extensions  map[string]any `json:"extensions,omitempty"`
}

type Variable struct {
	Key         string `json:"key"`
	Scope       string `json:"scope"`
	Value       any    `json:"value,omitempty"`
	SecretRef   string `json:"secretRef,omitempty"`
	Sensitive   bool   `json:"sensitive"`
	Description string `json:"description,omitempty"`
}

type Step struct {
	Key             string           `json:"key"`
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	DependsOn       []string         `json:"dependsOn"`
	TimeoutMS       int              `json:"timeoutMs"`
	FailurePolicy   string           `json:"failurePolicy"`
	APIOperationRef *APIOperationRef `json:"apiOperationRef,omitempty"`
	Request         *Request         `json:"request,omitempty"`
	Script          *Script          `json:"script,omitempty"`
	DelayMS         *int             `json:"delayMs,omitempty"`
	Extractors      []Extractor      `json:"extractors,omitempty"`
	Assertions      []Assertion      `json:"assertions,omitempty"`
	Extensions      map[string]any   `json:"extensions,omitempty"`
}

type APIOperationRef struct {
	OperationKey string `json:"operationKey"`
	ContentHash  string `json:"contentHash,omitempty"`
}

type Request struct {
	ConnectorRef string            `json:"connectorRef"`
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers,omitempty"`
	Query        map[string]any    `json:"query,omitempty"`
	Body         any               `json:"body,omitempty"`
	AuthRef      string            `json:"authRef,omitempty"`
}

type Script struct {
	Language       string `json:"language"`
	Source         string `json:"source"`
	RequiresReview bool   `json:"requiresReview"`
}

type Extractor struct {
	Key        string `json:"key"`
	Source     string `json:"source"`
	Expression string `json:"expression"`
	Required   bool   `json:"required,omitempty"`
}

type Assertion struct {
	Key      string `json:"key"`
	Type     string `json:"type"`
	Operator string `json:"operator"`
	Actual   string `json:"actual"`
	Expected any    `json:"expected,omitempty"`
	Message  string `json:"message,omitempty"`
}

type postmanCollection struct {
	Info struct {
		Name        string `json:"name"`
		Description any    `json:"description"`
		Schema      string `json:"schema"`
	} `json:"info"`
	Variables []postmanVariable `json:"variable"`
	Events    []postmanEvent    `json:"event"`
	Items     []postmanItem     `json:"item"`
}

type postmanVariable struct {
	Key      string `json:"key"`
	Value    any    `json:"value"`
	Disabled bool   `json:"disabled"`
}

type postmanItem struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Items   []postmanItem   `json:"item"`
	Request json.RawMessage `json:"request"`
	Events  []postmanEvent  `json:"event"`
}

type postmanEvent struct {
	Listen string `json:"listen"`
	Script struct {
		Exec json.RawMessage `json:"exec"`
	} `json:"script"`
}

type postmanRequest struct {
	Method string `json:"method"`
	URL    any    `json:"url"`
	Header []struct {
		Key      string `json:"key"`
		Value    string `json:"value"`
		Disabled bool   `json:"disabled"`
	} `json:"header"`
	Body *struct {
		Mode string `json:"mode"`
		Raw  string `json:"raw"`
	} `json:"body"`
}
