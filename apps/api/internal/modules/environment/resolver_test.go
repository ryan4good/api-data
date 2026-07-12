package environment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const (
	sysA   = "11111111-1111-4111-8111-111111111111"
	sysB   = "22222222-2222-4222-8222-222222222222"
	envID  = "33333333-3333-4333-8333-333333333333"
	userID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

type rotatingProvider struct {
	value string
	err   error
}

func (p *rotatingProvider) Resolve(context.Context, string) (string, error) { return p.value, p.err }
func fixture() (*MemoryRepository, Environment) {
	r := NewMemoryRepository()
	e := Environment{ID: envID, SystemID: sysA, Key: "test", Name: "Test", Variables: map[string]string{"baseUrl": "https://api.test"}, Status: StatusActive, CreatedBy: userID}
	_ = r.UpsertEnvironment(context.Background(), e)
	_ = r.UpsertSecretReference(context.Background(), SecretReference{ID: "44444444-4444-4444-8444-444444444444", SystemID: sysA, EnvironmentID: envID, VariableKey: "token", SecretRef: "vault://test/token", CreatedBy: userID})
	return r, e
}
func TestResolverMergesVariablesAndUsesCurrentProviderValue(t *testing.T) {
	r, _ := fixture()
	p := &rotatingProvider{value: "first-secret"}
	resolver := NewResolver(r, p)
	first, err := resolver.Resolve(context.Background(), sysA, envID, []string{"api.test", "shared.test"}, []string{"api.test", "evil.test"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Variables["token"] != "first-secret" || len(first.AllowedHosts) != 1 || first.AllowedHosts[0] != "api.test" {
		t.Fatalf("first=%#v", first)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "first-secret") || strings.Contains(fmt.Sprintf("%v %#v", first, first), "first-secret") {
		t.Fatal("resolved secret leaked through JSON or formatting")
	}
	p.value = "rotated-secret"
	second, err := resolver.Resolve(context.Background(), sysA, envID, nil, nil)
	if err != nil || second.Variables["token"] != "rotated-secret" {
		t.Fatalf("second=%#v err=%v", second, err)
	}
}
func TestResolverDoesNotLeakProviderSecretOrError(t *testing.T) {
	r, _ := fixture()
	providerErr := errors.New("vault failed for plaintext-super-secret")
	resolver := NewResolver(r, &rotatingProvider{err: providerErr})
	_, err := resolver.Resolve(context.Background(), sysA, envID, nil, nil)
	if !errors.Is(err, ErrSecretUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "plaintext-super-secret") || strings.Contains(err.Error(), "vault failed") {
		t.Fatalf("leaked err=%q", err.Error())
	}
}
func TestResolverRejectsDisabledAndCrossSystemEnvironment(t *testing.T) {
	r, e := fixture()
	e.Status = StatusDisabled
	_ = r.UpsertEnvironment(context.Background(), e)
	resolver := NewResolver(r, &rotatingProvider{})
	if _, err := resolver.Resolve(context.Background(), sysA, envID, nil, nil); !errors.Is(err, ErrEnvironmentDisabled) {
		t.Fatalf("disabled err=%v", err)
	}
	if _, err := resolver.Resolve(context.Background(), sysB, envID, nil, nil); !errors.Is(err, ErrEnvironmentNotFound) {
		t.Fatalf("cross err=%v", err)
	}
}
