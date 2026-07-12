package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bizdevops/apps/api/internal/modules/execution"
)

var (
	ErrUnsupportedScheme    = errors.New("connector URL scheme must be http or https")
	ErrTargetNotAllowed     = errors.New("connector target host is not allowed")
	ErrPrivateTarget        = errors.New("connector target resolves to a private or special-use address")
	ErrRequestBodyTooLarge  = errors.New("connector request body is too large")
	ErrResponseBodyTooLarge = errors.New("connector response body is too large")
	ErrRequestTimeout       = errors.New("connector request timed out")
	ErrUnboundVariable      = errors.New("connector variable is not bound")
	ErrInvalidStepConfig    = errors.New("invalid HTTP step configuration")
	ErrExtractorMissing     = errors.New("required extractor value is missing")
)

var variableExpression = regexp.MustCompile(`\{\{\s*([A-Za-z][A-Za-z0-9._-]*)\s*\}\}`)

type HTTPExecutorConfig struct {
	AllowedHosts         []string
	AllowPrivate         bool
	MaxRequestBodyBytes  int64
	MaxResponseBodyBytes int64
	DefaultTimeout       time.Duration
	MaxTimeout           time.Duration
}

type HTTPExecutor struct {
	allowedHosts         map[string]struct{}
	allowPrivate         bool
	maxRequestBodyBytes  int64
	maxResponseBodyBytes int64
	defaultTimeout       time.Duration
	maxTimeout           time.Duration
	client               *http.Client
}

func NewHTTPExecutor(config HTTPExecutorConfig) (*HTTPExecutor, error) {
	allowed := make(map[string]struct{}, len(config.AllowedHosts))
	for _, host := range config.AllowedHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			allowed[host] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil, fmt.Errorf("%w: allowed hosts are required", ErrInvalidStepConfig)
	}
	if config.MaxRequestBodyBytes <= 0 {
		config.MaxRequestBodyBytes = 1 << 20
	}
	if config.MaxResponseBodyBytes <= 0 {
		config.MaxResponseBodyBytes = 1 << 20
	}
	if config.DefaultTimeout <= 0 {
		config.DefaultTimeout = 30 * time.Second
	}
	if config.MaxTimeout <= 0 {
		config.MaxTimeout = 60 * time.Second
	}
	if config.DefaultTimeout > config.MaxTimeout {
		config.DefaultTimeout = config.MaxTimeout
	}
	executor := &HTTPExecutor{
		allowedHosts: allowed, allowPrivate: config.AllowPrivate,
		maxRequestBodyBytes: config.MaxRequestBodyBytes, maxResponseBodyBytes: config.MaxResponseBodyBytes,
		defaultTimeout: config.DefaultTimeout, maxTimeout: config.MaxTimeout,
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = executor.safeDialContext
	executor.client = &http.Client{Transport: transport, CheckRedirect: func(request *http.Request, _ []*http.Request) error {
		return executor.validateTarget(request.Context(), request.URL)
	}}
	return executor, nil
}

func (executor *HTTPExecutor) Execute(ctx context.Context, step execution.Step, variables map[string]any) (execution.StepResult, error) {
	started := time.Now()
	config := step.Config
	if step.Type != "" && step.Type != "http" {
		return execution.StepResult{}, ErrInvalidStepConfig
	}
	timeout := executor.stepTimeout(config["timeoutMs"])
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	baseURL, ok := stringValue(config["baseURL"])
	if !ok || baseURL == "" {
		return execution.StepResult{}, ErrInvalidStepConfig
	}
	path, _ := stringValue(config["path"])
	renderedBase, err := renderString(baseURL, variables)
	if err != nil {
		return execution.StepResult{}, err
	}
	renderedPath, err := renderString(path, variables)
	if err != nil {
		return execution.StepResult{}, err
	}
	target, err := joinTarget(renderedBase, renderedPath)
	if err != nil {
		return execution.StepResult{}, ErrInvalidStepConfig
	}
	if err := executor.validateTarget(requestCtx, target); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return execution.StepResult{}, ErrRequestTimeout
		}
		return execution.StepResult{}, err
	}
	query, err := renderStringMap(config["query"], variables)
	if err != nil {
		return execution.StepResult{}, err
	}
	values := target.Query()
	for key, value := range query {
		values.Set(key, value)
	}
	target.RawQuery = values.Encode()
	headers, err := renderStringMap(config["headers"], variables)
	if err != nil {
		return execution.StepResult{}, err
	}
	renderedBody, err := renderValue(config["body"], variables)
	if err != nil {
		return execution.StepResult{}, err
	}
	body, err := encodeRequestBody(renderedBody)
	if err != nil {
		return execution.StepResult{}, ErrInvalidStepConfig
	}
	if int64(len(body)) > executor.maxRequestBodyBytes {
		return execution.StepResult{}, ErrRequestBodyTooLarge
	}
	method, _ := stringValue(config["method"])
	if method == "" {
		method = http.MethodGet
	}
	request, err := http.NewRequestWithContext(requestCtx, strings.ToUpper(method), target.String(), bytes.NewReader(body))
	if err != nil {
		return execution.StepResult{}, ErrInvalidStepConfig
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := executor.client.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return execution.StepResult{}, ErrRequestTimeout
		}
		return execution.StepResult{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, executor.maxResponseBodyBytes+1))
	if err != nil {
		return execution.StepResult{}, err
	}
	if int64(len(responseBody)) > executor.maxResponseBodyBytes {
		return execution.StepResult{}, ErrResponseBodyTooLarge
	}
	parsedBody := parseResponseBody(responseBody)
	extracted, err := evaluateExtractors(config["extractors"], parsedBody)
	if err != nil {
		return execution.StepResult{}, err
	}
	assertions := evaluateAssertions(config["assertions"], response.StatusCode, parsedBody)
	duration := int(time.Since(started).Milliseconds())
	requestURL := target.Scheme + "://" + target.Host + sanitizeKnownValues(target.Path, variables)
	requestSnapshot := redactSnapshotValue(map[string]any{
		"method": request.Method, "url": requestURL, "headers": redactStringMap(headers), "query": redactStringMap(query), "body": redactValue(renderedBody), "sizeBytes": len(body),
	}, variables).(map[string]any)
	responseSnapshot := redactSnapshotValue(map[string]any{
		"status": response.StatusCode, "headers": redactHTTPHeader(response.Header), "body": redactValue(parsedBody), "sizeBytes": len(responseBody),
	}, variables).(map[string]any)
	for index := range assertions {
		assertions[index].Expected = redactSnapshotValue(assertions[index].Expected, variables)
		assertions[index].Actual = redactSnapshotValue(assertions[index].Actual, variables)
		assertions[index].Message = sanitizeKnownValues(assertions[index].Message, variables)
	}
	return execution.StepResult{
		RequestSnapshot:    requestSnapshot,
		ResponseSnapshot:   responseSnapshot,
		ExtractedVariables: extracted, Assertions: assertions, DurationMS: duration,
	}, nil
}

func (executor *HTTPExecutor) safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrTargetNotAllowed
	}
	if _, ok := executor.allowedHosts[strings.ToLower(host)]; !ok {
		return nil, ErrTargetNotAllowed
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{}
	for _, resolved := range addresses {
		if !executor.allowPrivate && unsafeIP(resolved.IP) {
			continue
		}
		connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		err = dialErr
	}
	if err != nil {
		return nil, err
	}
	return nil, ErrPrivateTarget
}

func (executor *HTTPExecutor) validateTarget(ctx context.Context, target *url.URL) error {
	if target.Scheme != "http" && target.Scheme != "https" {
		return ErrUnsupportedScheme
	}
	host := strings.ToLower(target.Hostname())
	if host == "" {
		return ErrTargetNotAllowed
	}
	if _, ok := executor.allowedHosts[host]; !ok {
		return ErrTargetNotAllowed
	}
	if executor.allowPrivate {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if unsafeIP(ip) {
			return ErrPrivateTarget
		}
		return nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	if len(addresses) == 0 {
		return ErrTargetNotAllowed
	}
	for _, address := range addresses {
		if unsafeIP(address.IP) {
			return ErrPrivateTarget
		}
	}
	return nil
}

func unsafeIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

func (executor *HTTPExecutor) stepTimeout(value any) time.Duration {
	timeout := executor.defaultTimeout
	if milliseconds, ok := numberValue(value); ok && milliseconds > 0 {
		timeout = time.Duration(milliseconds) * time.Millisecond
	}
	if timeout > executor.maxTimeout {
		timeout = executor.maxTimeout
	}
	return timeout
}

func joinTarget(base, path string) (*url.URL, error) {
	target, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return target, nil
	}
	reference, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	return target.ResolveReference(reference), nil
}

func renderString(value string, variables map[string]any) (string, error) {
	var renderErr error
	rendered := variableExpression.ReplaceAllStringFunc(value, func(match string) string {
		parts := variableExpression.FindStringSubmatch(match)
		variable, ok := variables[parts[1]]
		if !ok {
			renderErr = fmt.Errorf("%w: %s", ErrUnboundVariable, parts[1])
			return ""
		}
		return fmt.Sprint(variable)
	})
	return rendered, renderErr
}

func renderValue(value any, variables map[string]any) (any, error) {
	switch typed := value.(type) {
	case string:
		matches := variableExpression.FindStringSubmatch(typed)
		if len(matches) > 0 && matches[0] == typed {
			if value, ok := variables[matches[1]]; ok {
				return value, nil
			}
			return nil, fmt.Errorf("%w: %s", ErrUnboundVariable, matches[1])
		}
		return renderString(typed, variables)
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			rendered, err := renderValue(item, variables)
			if err != nil {
				return nil, err
			}
			result[key] = rendered
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			rendered, err := renderValue(item, variables)
			if err != nil {
				return nil, err
			}
			result[index] = rendered
		}
		return result, nil
	default:
		return value, nil
	}
}

func renderStringMap(value any, variables map[string]any) (map[string]string, error) {
	result := map[string]string{}
	switch typed := value.(type) {
	case nil:
		return result, nil
	case map[string]any:
		for key, item := range typed {
			rendered, err := renderString(fmt.Sprint(item), variables)
			if err != nil {
				return nil, err
			}
			result[key] = rendered
		}
	case map[string]string:
		for key, item := range typed {
			rendered, err := renderString(item, variables)
			if err != nil {
				return nil, err
			}
			result[key] = rendered
		}
	default:
		return nil, ErrInvalidStepConfig
	}
	return result, nil
}

func encodeRequestBody(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	if text, ok := value.(string); ok {
		return []byte(text), nil
	}
	return json.Marshal(value)
}

func parseResponseBody(data []byte) any {
	if len(data) == 0 {
		return nil
	}
	var value any
	if json.Unmarshal(data, &value) == nil {
		return value
	}
	return string(data)
}

func evaluateExtractors(config any, body any) (map[string]any, error) {
	result := map[string]any{}
	for _, item := range objectList(config) {
		key, _ := stringValue(item["key"])
		path, _ := stringValue(item["path"])
		if path == "" {
			path, _ = stringValue(item["expression"])
		}
		path = normalizeDotPath(path)
		value, found := dotPath(body, path)
		if !found {
			if required, _ := item["required"].(bool); required {
				return nil, fmt.Errorf("%w: %s", ErrExtractorMissing, key)
			}
			continue
		}
		if key != "" {
			result[key] = value
		}
	}
	return result, nil
}

func evaluateAssertions(config any, status int, body any) []execution.AssertionResult {
	items := objectList(config)
	results := make([]execution.AssertionResult, 0, len(items))
	for index, item := range items {
		key, _ := stringValue(item["key"])
		if key == "" {
			key = fmt.Sprintf("assertion-%d", index+1)
		}
		assertionType, _ := stringValue(item["type"])
		operator, _ := stringValue(item["operator"])
		if operator == "" {
			operator = "eq"
		}
		expected := item["expected"]
		var actual any
		found := true
		path, _ := stringValue(item["path"])
		if path == "" {
			path, _ = stringValue(item["actual"])
		}
		path = normalizeDotPath(path)
		if assertionType == "status" {
			actual = status
		} else {
			actual, found = dotPath(body, path)
		}
		passed := found && compareAssertion(actual, expected, operator)
		result := execution.AssertionResult{Key: key, Type: assertionType, Status: execution.AssertionFailed, Expected: expected, Actual: actual}
		if passed {
			result.Status = execution.AssertionPassed
		} else {
			result.Message = "assertion did not match"
		}
		if sensitiveKey(key) || sensitiveKey(path) {
			result.Expected, result.Actual = "[REDACTED]", "[REDACTED]"
		}
		results = append(results, result)
	}
	return results
}

func normalizeDotPath(path string) string {
	path = strings.TrimSpace(path)
	for _, prefix := range []string{"$body.", "$."} {
		if strings.HasPrefix(path, prefix) {
			return strings.TrimPrefix(path, prefix)
		}
	}
	if path == "$" || path == "$body" {
		return ""
	}
	return path
}

func objectList(value any) []map[string]any {
	items, _ := value.([]any)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if object, ok := item.(map[string]any); ok {
			result = append(result, object)
		}
	}
	return result
}

func dotPath(value any, path string) (any, bool) {
	if path == "" {
		return value, true
	}
	current := value
	for _, part := range strings.Split(path, ".") {
		switch typed := current.(type) {
		case map[string]any:
			current, _ = typed[part]
			if current == nil {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func compareAssertion(actual, expected any, operator string) bool {
	switch operator {
	case "eq":
		return normalizeValue(actual) == normalizeValue(expected)
	case "ne":
		return normalizeValue(actual) != normalizeValue(expected)
	case "contains":
		return strings.Contains(fmt.Sprint(actual), fmt.Sprint(expected))
	default:
		return false
	}
}

func normalizeValue(value any) string {
	if number, ok := numberValue(value); ok {
		return strconv.FormatInt(number, 10)
	}
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func numberValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), typed == float64(int64(typed))
	case json.Number:
		number, err := typed.Int64()
		return number, err == nil
	default:
		return 0, false
	}
}

func stringValue(value any) (string, bool) {
	text, ok := value.(string)
	return text, ok
}

func redactStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		if sensitiveKey(key) {
			result[key] = "[REDACTED]"
		} else {
			result[key] = value
		}
	}
	return result
}

func redactHTTPHeader(source http.Header) map[string]any {
	result := make(map[string]any, len(source))
	for key, values := range source {
		if sensitiveKey(key) {
			result[key] = "[REDACTED]"
		} else {
			result[key] = append([]string(nil), values...)
		}
	}
	return result
}

func redactValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if sensitiveKey(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = redactValue(item)
			}
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = redactValue(item)
		}
		return result
	default:
		return typed
	}
}

func sensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, marker := range []string{"authorization", "cookie", "password", "passwd", "secret", "token", "api_key", "apikey"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func sanitizeKnownValues(value string, variables map[string]any) string {
	for key, item := range variables {
		if sensitiveKey(key) {
			if text, ok := item.(string); ok && text != "" {
				value = strings.ReplaceAll(value, text, "[REDACTED]")
			}
		}
	}
	return value
}

func redactSnapshotValue(value any, variables map[string]any) any {
	switch typed := value.(type) {
	case string:
		return sanitizeKnownValues(typed, variables)
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if sensitiveKey(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = redactSnapshotValue(item, variables)
			}
		}
		return result
	case map[string]string:
		result := make(map[string]string, len(typed))
		for key, item := range typed {
			if sensitiveKey(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = sanitizeKnownValues(item, variables)
			}
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = redactSnapshotValue(item, variables)
		}
		return result
	case []string:
		result := make([]string, len(typed))
		for index, item := range typed {
			result[index] = sanitizeKnownValues(item, variables)
		}
		return result
	default:
		return typed
	}
}

var _ execution.StepExecutor = (*HTTPExecutor)(nil)
