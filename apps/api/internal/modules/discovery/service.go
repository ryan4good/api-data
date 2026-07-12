package discovery

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"bizdevops/apps/api/internal/modules/scanner"
)

type DiscoverInput struct {
	SystemID     string
	CodeSourceID string
	Type         Type
	Name         string
	PRD          string
	Prompt       string
	Operations   []scanner.APIOperation
	RequestedBy  string
}

type Service struct {
	repository Repository
	now        func() time.Time
	newID      func() (string, error)
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now, newID: newDiscoveryUUID}
}

func (s *Service) Discover(ctx context.Context, input DiscoverInput) (Discovery, []Candidate, error) {
	if input.SystemID == "" || input.RequestedBy == "" || strings.TrimSpace(input.Name) == "" || !input.Type.Valid() {
		return Discovery{}, nil, fmt.Errorf("%w: valid systemID, requestedBy, name and type are required", ErrInvalidInput)
	}
	if err := validateDiscoveryInput(input); err != nil {
		return Discovery{}, nil, err
	}
	id, err := s.newID()
	if err != nil {
		return Discovery{}, nil, err
	}
	now := s.now().UTC()
	inputDoc, _ := json.Marshal(map[string]any{"prd": input.PRD, "prompt": input.Prompt, "operationIds": operationIDs(input.Operations)})
	discovery := Discovery{ID: id, SystemID: input.SystemID, CodeSourceID: input.CodeSourceID, Type: input.Type, Name: input.Name, InputDocument: inputDoc, Status: StatusReady, RequestedBy: input.RequestedBy, StartedAt: &now, FinishedAt: &now, CreatedAt: now, UpdatedAt: now}
	candidates, err := s.generate(input, id, now)
	if err != nil {
		return Discovery{}, nil, err
	}
	discovery.Summary, _ = json.Marshal(map[string]int{"candidateCount": len(candidates)})
	if err := s.repository.CreateWithCandidates(ctx, discovery, candidates); err != nil {
		return Discovery{}, nil, err
	}
	return discovery, candidates, nil
}

func validateDiscoveryInput(input DiscoverInput) error {
	for _, op := range input.Operations {
		if op.SystemID != input.SystemID {
			return fmt.Errorf("%w: operation system scope mismatch", ErrInvalidInput)
		}
	}
	switch input.Type {
	case TypeCode:
		if len(input.Operations) == 0 {
			return fmt.Errorf("%w: code discovery requires operations", ErrInvalidInput)
		}
	case TypePRD:
		if strings.TrimSpace(input.PRD) == "" {
			return fmt.Errorf("%w: prd discovery requires prd", ErrInvalidInput)
		}
	case TypePrompt:
		if strings.TrimSpace(input.Prompt) == "" {
			return fmt.Errorf("%w: prompt discovery requires prompt", ErrInvalidInput)
		}
	case TypeMixed:
		if len(input.Operations) == 0 && strings.TrimSpace(input.PRD) == "" && strings.TrimSpace(input.Prompt) == "" {
			return fmt.Errorf("%w: mixed discovery requires input", ErrInvalidInput)
		}
	}
	return nil
}

func (s *Service) generate(input DiscoverInput, discoveryID string, now time.Time) ([]Candidate, error) {
	ops := append([]scanner.APIOperation(nil), input.Operations...)
	sort.Slice(ops, func(i, j int) bool { return ops[i].OperationKey < ops[j].OperationKey })
	critical := containsCritical(input.Name + " " + input.PRD + " " + input.Prompt)
	writes := []scanner.APIOperation{}
	for _, op := range ops {
		if isWrite(op.Method) {
			writes = append(writes, op)
		}
	}
	if len(writes) == 0 && (critical || containsWriteIntent(input.PRD+" "+input.Prompt)) {
		id, err := s.newID()
		if err != nil {
			return nil, err
		}
		refs := documentRefs(input)
		bundle := CandidateBundle{Priority: PriorityP0, RequiresReview: true, SourceRefs: refs, Steps: []CandidateStep{{Key: "intent", Name: strings.TrimSpace(input.Name)}}}
		return []Candidate{buildCandidate(id, input.SystemID, discoveryID, "intent-p0", input.Name, .82, bundle, now)}, nil
	}
	out := make([]Candidate, 0, len(writes))
	for _, write := range writes {
		id, err := s.newID()
		if err != nil {
			return nil, err
		}
		resource := resourceOf(write.Path)
		steps := []CandidateStep{}
		if pre, ok := findQuery(ops, resource, false); ok {
			steps = append(steps, step("precondition", "前置查询", pre))
		}
		steps = append(steps, step("action", "执行写操作", write))
		if result, ok := findQuery(ops, resource, true); ok {
			steps = append(steps, step("result", "结果查询", result))
		}
		refs := []string{}
		for _, st := range steps {
			if st.OperationID != "" {
				refs = append(refs, "api-operation:"+st.OperationID)
			}
		}
		refs = append(refs, documentRefs(input)...)
		confidence := .8
		if critical || containsCritical(write.Path+" "+write.Summary) {
			confidence = .92
		}
		key := strings.ToLower(write.Method) + "-" + strings.Trim(resource, "/")
		key = strings.ReplaceAll(key, "/", "-")
		bundle := CandidateBundle{Priority: PriorityP0, RequiresReview: true, SourceRefs: refs, Steps: steps}
		out = append(out, buildCandidate(id, input.SystemID, discoveryID, key, input.Name+" - "+write.Method+" "+write.Path, confidence, bundle, now))
	}
	return out, nil
}

func buildCandidate(id, systemID, discoveryID, key, name string, confidence float64, bundle CandidateBundle, now time.Time) Candidate {
	raw, _ := json.Marshal(bundle)
	evidence, _ := json.Marshal(map[string]any{"sourceRefs": bundle.SourceRefs})
	return Candidate{ID: id, SystemID: systemID, DiscoveryID: discoveryID, Key: key, Name: name, Priority: bundle.Priority, Confidence: confidence, SourceRefs: bundle.SourceRefs, RequiresReview: true, Steps: bundle.Steps, Evidence: evidence, Bundle: raw, ReviewStatus: ReviewPending, CreatedAt: now, UpdatedAt: now}
}
func step(key, name string, op scanner.APIOperation) CandidateStep {
	return CandidateStep{Key: key, Name: name, OperationID: op.ID, Method: op.Method, Path: op.Path}
}
func isWrite(m string) bool {
	switch strings.ToUpper(m) {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}
func containsCritical(v string) bool {
	v = strings.ToLower(v)
	for _, k := range []string{"order", "payment", "pay", "auth", "login", "订单", "支付", "认证", "登录"} {
		if strings.Contains(v, k) {
			return true
		}
	}
	return false
}
func containsWriteIntent(v string) bool {
	v = strings.ToLower(v)
	for _, k := range []string{"create", "update", "delete", "submit", "创建", "修改", "删除", "提交", "下单"} {
		if strings.Contains(v, k) {
			return true
		}
	}
	return false
}
func resourceOf(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "/"
	}
	return "/" + parts[0]
}
func findQuery(ops []scanner.APIOperation, resource string, detail bool) (scanner.APIOperation, bool) {
	for _, op := range ops {
		if strings.EqualFold(op.Method, "GET") && strings.HasPrefix(op.Path, resource) {
			isDetail := strings.Contains(op.Path, ":") || strings.Contains(op.Path, "{")
			if isDetail == detail {
				return op, true
			}
		}
	}
	return scanner.APIOperation{}, false
}
func documentRefs(input DiscoverInput) []string {
	out := []string{}
	if strings.TrimSpace(input.PRD) != "" {
		out = append(out, "prd")
	}
	if strings.TrimSpace(input.Prompt) != "" {
		out = append(out, "prompt")
	}
	return out
}
func operationIDs(ops []scanner.APIOperation) []string {
	out := make([]string, 0, len(ops))
	for _, op := range ops {
		out = append(out, op.ID)
	}
	return out
}

func (s *Service) ListDiscoveries(ctx context.Context, systemID string) ([]Discovery, error) {
	return s.repository.ListDiscoveries(ctx, systemID)
}
func (s *Service) ListCandidates(ctx context.Context, systemID, discoveryID string) ([]Candidate, error) {
	return s.repository.ListCandidates(ctx, systemID, discoveryID)
}
func (s *Service) Review(ctx context.Context, systemID, candidateID string, status ReviewStatus, note, reviewerID string) (Candidate, error) {
	if status != ReviewAccepted && status != ReviewRejected {
		return Candidate{}, errors.New("review must accept or reject")
	}
	return s.repository.ReviewCandidate(ctx, systemID, candidateID, status, note, reviewerID, s.now().UTC())
}
func (s *Service) AllCandidatesReviewed(ctx context.Context, systemID, discoveryID string) (bool, error) {
	items, err := s.repository.ListCandidates(ctx, systemID, discoveryID)
	if err != nil || len(items) == 0 {
		return false, err
	}
	for _, item := range items {
		if item.ReviewStatus == ReviewPending {
			return false, nil
		}
	}
	return true, nil
}
func newDiscoveryUUID() (string, error) {
	var v [16]byte
	if _, err := rand.Read(v[:]); err != nil {
		return "", err
	}
	v[6] = (v[6] & 0x0f) | 0x40
	v[8] = (v[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", v[0:4], v[4:6], v[6:8], v[8:10], v[10:16]), nil
}
