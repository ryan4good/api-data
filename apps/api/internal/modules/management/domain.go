package management

type Scope string

const (
	ScopeMember   Scope = "authorized"
	ScopePlatform Scope = "platform_admin"
)

type RunCounts struct {
	Passed  int `json:"succeeded"`
	Failed  int `json:"failed"`
	Running int `json:"running"`
}

type Risk struct {
	ID          string `json:"id"`
	Level       string `json:"level"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	SystemID    string `json:"systemId,omitempty"`
	SystemName  string `json:"systemName,omitempty"`
}

type SystemOverview struct {
	SystemID       string    `json:"systemId"`
	SystemKey      string    `json:"code"`
	SystemName     string    `json:"name"`
	MyRole         string    `json:"myRole"`
	APICount       int       `json:"apiAssetCount"`
	P0PendingCount int       `json:"p0CandidateCount"`
	ScenarioCount  int       `json:"scenarioCount"`
	Runs24h        RunCounts `json:"runs24h"`
	RiskCount      int       `json:"riskCount"`
	Risks          []Risk    `json:"risks"`
}

type Overview struct {
	Scope          Scope            `json:"accessScope"`
	SystemCount    int              `json:"systemCount"`
	APICount       int              `json:"apiAssetCount"`
	P0PendingCount int              `json:"p0CandidateCount"`
	ScenarioCount  int              `json:"scenarioCount"`
	Runs24h        RunCounts        `json:"runs24h"`
	Risks          []Risk           `json:"risks"`
	Systems        []SystemOverview `json:"systems"`
}

func buildOverview(scope Scope, systems []SystemOverview) Overview {
	result := Overview{Scope: scope, SystemCount: len(systems), Systems: systems, Risks: make([]Risk, 0)}
	for index := range result.Systems {
		item := &result.Systems[index]
		if item.Risks == nil {
			item.Risks = make([]Risk, 0)
			if item.P0PendingCount > 0 {
				item.Risks = append(item.Risks, Risk{ID: item.SystemID + ":p0-review", Level: "medium", Title: "P0 候选待核验", SystemID: item.SystemID, SystemName: item.SystemName})
			}
			if item.Runs24h.Failed > 0 {
				item.Risks = append(item.Risks, Risk{ID: item.SystemID + ":run-failure", Level: "high", Title: "近 24 小时存在失败运行", SystemID: item.SystemID, SystemName: item.SystemName})
			}
		}
		item.RiskCount = len(item.Risks)
		result.Risks = append(result.Risks, item.Risks...)
		result.APICount += item.APICount
		result.P0PendingCount += item.P0PendingCount
		result.ScenarioCount += item.ScenarioCount
		result.Runs24h.Passed += item.Runs24h.Passed
		result.Runs24h.Failed += item.Runs24h.Failed
		result.Runs24h.Running += item.Runs24h.Running
	}
	return result
}
