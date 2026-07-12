package connector

import "sort"

// NarrowHostAllowlist intersects connector-level hosts with the global policy.
// An empty connector policy inherits global; an empty global policy denies all.
func NarrowHostAllowlist(global, connector []string) []string {
	allowed := make(map[string]bool, len(global))
	for _, host := range global {
		allowed[host] = true
	}
	out := make([]string, 0)
	if len(connector) == 0 {
		for host := range allowed {
			out = append(out, host)
		}
	} else {
		seen := make(map[string]bool)
		for _, host := range connector {
			if allowed[host] && !seen[host] {
				out = append(out, host)
				seen[host] = true
			}
		}
	}
	sort.Strings(out)
	return out
}
