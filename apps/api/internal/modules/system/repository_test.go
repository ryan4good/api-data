package system

import (
	"context"
	"testing"
	"time"

	"bizdevops/apps/api/internal/modules/access"
)

func TestMemoryRepositoryScopesSystemsByActiveMembership(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository(
		[]BusinessSystem{
			{ID: "11111111-1111-4111-8111-111111111111", Code: "billing", Name: "Billing", Status: StatusActive, CreatedAt: now, UpdatedAt: now},
			{ID: "22222222-2222-4222-8222-222222222222", Code: "orders", Name: "Orders", Status: StatusActive, CreatedAt: now, UpdatedAt: now},
		},
		[]Member{
			{SystemID: "11111111-1111-4111-8111-111111111111", UserID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", DisplayName: "Alice", Role: access.RoleMaintainer, Status: MemberActive},
			{SystemID: "22222222-2222-4222-8222-222222222222", UserID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", DisplayName: "Alice", Role: access.RoleViewer, Status: MemberDisabled},
		},
	)

	got, err := repo.ListAuthorized(context.Background(), "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].System.ID != "11111111-1111-4111-8111-111111111111" || got[0].Role != access.RoleMaintainer {
		t.Fatalf("unexpected authorized systems: %#v", got)
	}
	if _, found, err := repo.GetAuthorized(context.Background(), "22222222-2222-4222-8222-222222222222", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"); err != nil || found {
		t.Fatalf("disabled membership must be outside scope: found=%v err=%v", found, err)
	}
}

func TestMemoryRepositoryUpsertsAndListsMembers(t *testing.T) {
	systemID := "11111111-1111-4111-8111-111111111111"
	repo := NewMemoryRepository([]BusinessSystem{{ID: systemID, Code: "billing", Name: "Billing", Status: StatusActive}}, nil)
	want := Member{SystemID: systemID, UserID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", DisplayName: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Role: access.RoleRunner, Status: MemberActive}

	if _, err := repo.UpsertMember(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	members, err := repo.ListMembers(context.Background(), systemID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0] != want {
		t.Fatalf("members=%#v, want %#v", members, []Member{want})
	}
}
