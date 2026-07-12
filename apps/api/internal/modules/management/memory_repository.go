package management

import (
	"context"
	"sort"
)

type MemoryRepository struct {
	systems     []SystemOverview
	memberships map[string]map[string]string
	admins      map[string]bool
}

func NewMemoryRepository(systems []SystemOverview, memberships map[string]map[string]string, admins map[string]bool) *MemoryRepository {
	return &MemoryRepository{systems: append([]SystemOverview(nil), systems...), memberships: memberships, admins: admins}
}

func (repository *MemoryRepository) Overview(_ context.Context, userID string) (Overview, error) {
	systems := make([]SystemOverview, 0)
	scope := ScopeMember
	if repository.admins[userID] {
		scope = ScopePlatform
		for _, item := range repository.systems {
			copy := item
			copy.MyRole = "platform_admin"
			systems = append(systems, copy)
		}
	} else {
		for _, item := range repository.systems {
			if role, found := repository.memberships[userID][item.SystemID]; found {
				copy := item
				copy.MyRole = role
				systems = append(systems, copy)
			}
		}
	}
	sort.Slice(systems, func(i, j int) bool { return systems[i].SystemName < systems[j].SystemName })
	return buildOverview(scope, systems), nil
}

func (repository *MemoryRepository) SystemOverview(_ context.Context, userID, systemID string) (SystemOverview, bool, error) {
	for _, item := range repository.systems {
		if item.SystemID != systemID {
			continue
		}
		if repository.admins[userID] {
			item.MyRole = "platform_admin"
			return item, true, nil
		}
		if role, found := repository.memberships[userID][systemID]; found {
			item.MyRole = role
			return item, true, nil
		}
		return SystemOverview{}, false, nil
	}
	return SystemOverview{}, false, nil
}

var _ Repository = (*MemoryRepository)(nil)
