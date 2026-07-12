package system

import (
	"context"
	"sort"
	"sync"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	systems map[string]BusinessSystem
	members map[string]map[string]Member
}

func NewMemoryRepository(systems []BusinessSystem, members []Member) *MemoryRepository {
	r := &MemoryRepository{systems: make(map[string]BusinessSystem), members: make(map[string]map[string]Member)}
	for _, item := range systems {
		r.systems[item.ID] = item
	}
	for _, member := range members {
		if r.members[member.SystemID] == nil {
			r.members[member.SystemID] = make(map[string]Member)
		}
		r.members[member.SystemID][member.UserID] = member
	}
	return r
}

func (r *MemoryRepository) ListAuthorized(_ context.Context, userID string) ([]AuthorizedSystem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]AuthorizedSystem, 0)
	for systemID, members := range r.members {
		member, ok := members[userID]
		system, exists := r.systems[systemID]
		if ok && exists && member.Status == MemberActive {
			result = append(result, AuthorizedSystem{System: system, Role: member.Role})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].System.Code < result[j].System.Code })
	return result, nil
}

func (r *MemoryRepository) GetAuthorized(_ context.Context, systemID, userID string) (AuthorizedSystem, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	system, exists := r.systems[systemID]
	member, memberExists := r.members[systemID][userID]
	if !exists || !memberExists || member.Status != MemberActive {
		return AuthorizedSystem{}, false, nil
	}
	return AuthorizedSystem{System: system, Role: member.Role}, true, nil
}

func (r *MemoryRepository) RoleForUser(_ context.Context, systemID, userID string) (string, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	member, ok := r.members[systemID][userID]
	if !ok || member.Status != MemberActive {
		return "", false, nil
	}
	return string(member.Role), true, nil
}

func (r *MemoryRepository) ListMembers(_ context.Context, systemID string) ([]Member, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Member, 0, len(r.members[systemID]))
	for _, member := range r.members[systemID] {
		result = append(result, member)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UserID < result[j].UserID })
	return result, nil
}

func (r *MemoryRepository) UpsertMember(_ context.Context, member Member) (Member, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.members[member.SystemID] == nil {
		r.members[member.SystemID] = make(map[string]Member)
	}
	r.members[member.SystemID][member.UserID] = member
	return member, nil
}
