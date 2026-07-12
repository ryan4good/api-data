package importer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var variablePattern = regexp.MustCompile(`\{\{\s*([A-Za-z][A-Za-z0-9._-]*)\s*\}\}`)
var postmanStatusPattern = regexp.MustCompile(`pm\.response\.to\.have\.status\s*\(\s*([0-9]{3})\s*\)`)

func Import(data []byte, options Options) (Result, error) {
	var discriminator struct {
		SchemaVersion string `json:"schemaVersion"`
		Info          *struct {
			Schema string `json:"schema"`
		} `json:"info"`
	}
	if err := json.Unmarshal(data, &discriminator); err != nil {
		result := Result{}
		result.Report.Errors = append(result.Report.Errors, Issue{Code: "INVALID_JSON", Message: err.Error()})
		return result, errors.New("invalid import JSON")
	}

	switch {
	case discriminator.SchemaVersion != "":
		return importNative(data)
	case discriminator.Info != nil && strings.Contains(discriminator.Info.Schema, "collection/v2.1"):
		return importPostman(data, options)
	default:
		result := Result{}
		result.Report.Errors = append(result.Report.Errors, Issue{Code: "UNSUPPORTED_FORMAT", Message: "expected Scenario Bundle 1.0 or Postman Collection 2.1"})
		return result, errors.New("unsupported import format")
	}
}

func importNative(data []byte) (Result, error) {
	result := Result{SourceFormat: FormatScenarioBundle}
	if err := decodeStrict(data, &result.Bundle); err != nil {
		result.Report.Errors = append(result.Report.Errors, Issue{Code: "INVALID_SCENARIO_BUNDLE", Message: err.Error()})
		return result, errors.New("invalid Scenario Bundle")
	}
	if result.Bundle.SchemaVersion != "1.0" || result.Bundle.System.Key == "" || result.Bundle.System.Name == "" ||
		result.Bundle.Scenario.Key == "" || result.Bundle.Scenario.Name == "" || len(result.Bundle.Scenario.Steps) == 0 {
		result.Report.Errors = append(result.Report.Errors, Issue{Code: "INVALID_SCENARIO_BUNDLE", Message: "required Bundle 1.0 metadata or steps are missing"})
		return result, errors.New("invalid Scenario Bundle")
	}
	result.Report.Counts.Variables = len(result.Bundle.Scenario.Variables)
	for index := range result.Bundle.Scenario.Variables {
		variable := &result.Bundle.Scenario.Variables[index]
		if variable.Sensitive && variable.Value != nil {
			variable.Value = nil
			if variable.SecretRef == "" {
				variable.SecretRef = "unbound://" + variable.Key
			}
			result.Report.Errors = append(result.Report.Errors, Issue{Code: "SENSITIVE_VALUE_REJECTED", Message: "sensitive variables cannot contain plaintext values", SourcePath: fmt.Sprintf("scenario.variables[%d]", index)})
		}
	}
	countSteps(&result)
	detectUnbound(&result)
	if len(result.Report.Errors) > 0 {
		return result, errors.New("Scenario Bundle contains blocking import errors")
	}
	return result, nil
}

func importPostman(data []byte, options Options) (Result, error) {
	var collection postmanCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return Result{}, err
	}
	systemKey := options.SystemKey
	if systemKey == "" {
		systemKey = "imported-system"
	}
	systemName := options.SystemName
	if systemName == "" {
		systemName = systemKey
	}
	connectorRef := options.ConnectorRef
	if connectorRef == "" {
		connectorRef = "default-http"
	}

	result := Result{
		SourceFormat: FormatPostmanCollection21,
		Bundle: Bundle{
			SchemaVersion: "1.0",
			ExportedAt:    time.Now().UTC().Format(time.RFC3339),
			System:        System{Key: safeKey(systemKey, "imported-system"), Name: systemName},
			Scenario: Scenario{
				Key:       safeKey(collection.Info.Name, "imported-scenario"),
				Name:      fallback(collection.Info.Name, "Imported scenario"),
				Version:   1,
				Variables: []Variable{},
				Steps:     []Step{},
			},
		},
	}
	for index, postmanVariable := range collection.Variables {
		if postmanVariable.Disabled || postmanVariable.Key == "" {
			continue
		}
		variableKey := postmanVariable.Key
		if !validContractKey(variableKey) {
			variableKey = safeKey(postmanVariable.Key, fmt.Sprintf("variable-%d", index+1))
		}
		variable := Variable{Key: variableKey, Scope: "scenario", Value: postmanVariable.Value}
		if isSecretName(postmanVariable.Key) {
			variable.Sensitive = true
			variable.Value = nil
			variable.SecretRef = "unbound://" + variable.Key
			result.Report.Warnings = append(result.Report.Warnings, Issue{Code: "SECRET_VALUE_REDACTED", Message: "suspected secret value was discarded; bind a secret reference before execution", SourcePath: fmt.Sprintf("variable[%d]", index)})
		}
		result.Bundle.Scenario.Variables = append(result.Bundle.Scenario.Variables, variable)
	}
	result.Report.Counts.Variables = len(result.Bundle.Scenario.Variables)

	builder := postmanBuilder{result: &result, connectorRef: connectorRef, usedKeys: map[string]int{}}
	builder.addEvents(collection.Events, "prerequest", "collection")
	for index := range collection.Items {
		builder.addItem(collection.Items[index], []string{fmt.Sprintf("item[%d]", index)})
	}
	builder.addEvents(collection.Events, "test", "collection")
	detectUnbound(&result)
	return result, nil
}

type postmanBuilder struct {
	result       *Result
	connectorRef string
	usedKeys     map[string]int
}

func (builder *postmanBuilder) addItem(item postmanItem, path []string) {
	sourcePath := strings.Join(path, ".")
	if len(item.Items) > 0 {
		builder.addEvents(item.Events, "prerequest", sourcePath)
		for index := range item.Items {
			builder.addItem(item.Items[index], append(path, fmt.Sprintf("item[%d]", index)))
		}
		builder.addEvents(item.Events, "test", sourcePath)
		return
	}
	if len(bytes.TrimSpace(item.Request)) == 0 || bytes.Equal(bytes.TrimSpace(item.Request), []byte("null")) {
		return
	}
	builder.addEvents(item.Events, "prerequest", sourcePath)
	var request postmanRequest
	if err := json.Unmarshal(item.Request, &request); err != nil {
		builder.result.Report.Errors = append(builder.result.Report.Errors, Issue{Code: "INVALID_REQUEST", Message: err.Error(), SourcePath: sourcePath + ".request"})
		return
	}
	method := strings.ToUpper(request.Method)
	if method == "" {
		method = "GET"
	}
	headers := map[string]string{}
	for _, header := range request.Header {
		if !header.Disabled && header.Key != "" {
			headers[header.Key] = header.Value
		}
	}
	httpRequest := &Request{ConnectorRef: builder.connectorRef, Method: method, URL: postmanURL(request.URL), Headers: headers}
	if request.Body != nil && request.Body.Mode == "raw" {
		var body any
		if json.Unmarshal([]byte(request.Body.Raw), &body) == nil {
			httpRequest.Body = body
		} else {
			httpRequest.Body = request.Body.Raw
		}
	}
	step := Step{
		Key:           builder.uniqueKey(fallback(item.ID, item.Name), "request"),
		Name:          fallback(item.Name, "Imported request"),
		Type:          "http",
		DependsOn:     builder.previousDependency(),
		TimeoutMS:     30000,
		FailurePolicy: "stop",
		Request:       httpRequest,
		Extensions:    map[string]any{"postmanSourcePath": sourcePath},
	}
	builder.result.Bundle.Scenario.Steps = append(builder.result.Bundle.Scenario.Steps, step)
	builder.result.Report.Counts.Requests++
	builder.addEvents(item.Events, "test", sourcePath)
}

func (builder *postmanBuilder) addEvents(events []postmanEvent, listen, sourcePath string) {
	for index, event := range events {
		if event.Listen != listen {
			continue
		}
		source := scriptSource(event.Script.Exec)
		if strings.TrimSpace(source) == "" {
			continue
		}
		if listen == "test" {
			builder.convertRecognizedTests(source)
		}
		step := Step{
			Key:           builder.uniqueKey(sourcePath+"-"+listen, "script"),
			Name:          "Postman " + listen + " script",
			Type:          "script",
			DependsOn:     builder.previousDependency(),
			TimeoutMS:     30000,
			FailurePolicy: "stop",
			Script:        &Script{Language: "javascript", Source: source, RequiresReview: true},
			Extensions:    map[string]any{"postmanSourcePath": fmt.Sprintf("%s.event[%d]", sourcePath, index), "postmanListen": listen},
		}
		builder.result.Bundle.Scenario.Steps = append(builder.result.Bundle.Scenario.Steps, step)
		builder.result.Report.Counts.Scripts++
		builder.result.Report.Warnings = append(builder.result.Report.Warnings, Issue{Code: "SCRIPT_REQUIRES_REVIEW", Message: "Postman scripts are preserved but must be reviewed before execution", SourcePath: fmt.Sprintf("%s.event[%d]", sourcePath, index)})
	}
}

func (builder *postmanBuilder) convertRecognizedTests(source string) {
	steps := builder.result.Bundle.Scenario.Steps
	requestIndex := -1
	for index := len(steps) - 1; index >= 0; index-- {
		if steps[index].Type == "http" {
			requestIndex = index
			break
		}
	}
	if requestIndex < 0 {
		return
	}
	for _, match := range postmanStatusPattern.FindAllStringSubmatch(source, -1) {
		status, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		assertion := Assertion{
			Key:      fmt.Sprintf("status-%d", status),
			Type:     "status",
			Operator: "eq",
			Actual:   "$status",
			Expected: status,
		}
		builder.result.Bundle.Scenario.Steps[requestIndex].Assertions = append(builder.result.Bundle.Scenario.Steps[requestIndex].Assertions, assertion)
		builder.result.Report.Counts.Assertions++
	}
}

func (builder *postmanBuilder) previousDependency() []string {
	steps := builder.result.Bundle.Scenario.Steps
	if len(steps) == 0 {
		return []string{}
	}
	return []string{steps[len(steps)-1].Key}
}

func (builder *postmanBuilder) uniqueKey(candidate, fallbackValue string) string {
	base := safeKey(candidate, fallbackValue)
	builder.usedKeys[base]++
	if builder.usedKeys[base] == 1 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, builder.usedKeys[base])
}

func countSteps(result *Result) {
	for _, step := range result.Bundle.Scenario.Steps {
		switch step.Type {
		case "http":
			result.Report.Counts.Requests++
		case "script":
			result.Report.Counts.Scripts++
		}
		result.Report.Counts.Assertions += len(step.Assertions)
	}
}

func detectUnbound(result *Result) {
	bound := make(map[string]bool, len(result.Bundle.Scenario.Variables))
	for _, variable := range result.Bundle.Scenario.Variables {
		bound[variable.Key] = true
	}
	seen := map[string]bool{}
	for index, step := range result.Bundle.Scenario.Steps {
		encoded, _ := json.Marshal(step)
		for _, match := range variablePattern.FindAllSubmatch(encoded, -1) {
			key := string(match[1])
			if !bound[key] && !seen[key] {
				seen[key] = true
				result.Report.Warnings = append(result.Report.Warnings, Issue{Code: "UNBOUND_VARIABLE", Message: fmt.Sprintf("variable %q has no imported binding", key), SourcePath: fmt.Sprintf("scenario.steps[%d]", index)})
			}
		}
		for _, extractor := range step.Extractors {
			bound[extractor.Key] = true
		}
	}
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("multiple JSON values")
	}
	return nil
}

func postmanURL(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		if raw, ok := typed["raw"].(string); ok {
			return raw
		}
	}
	return ""
}

func scriptSource(raw json.RawMessage) string {
	var lines []string
	if json.Unmarshal(raw, &lines) == nil {
		return strings.Join(lines, "\n")
	}
	var source string
	if json.Unmarshal(raw, &source) == nil {
		return source
	}
	return ""
}

func safeKey(value, fallbackValue string) string {
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(unicode.ToLower(char))
			lastDash = false
		} else if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	key := strings.Trim(builder.String(), "-")
	if key == "" || !isASCIIAlpha(key[0]) {
		key = fallbackValue
	}
	if len(key) > 120 {
		key = strings.TrimRight(key[:120], "-")
	}
	return key
}

func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func validContractKey(value string) bool {
	if len(value) == 0 || len(value) > 128 || !isASCIIAlpha(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		char := value[index]
		if !isASCIIAlpha(char) && !(char >= '0' && char <= '9') && char != '.' && char != '_' && char != '-' {
			return false
		}
	}
	return true
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}

func isSecretName(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range []string{"password", "passwd", "token", "secret", "api_key", "apikey", "authorization"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
