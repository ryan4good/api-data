package execution

import (
	"context"
	"database/sql"
	"encoding/json"
)

const selectScenarioStepsSQL = `SELECT id, position, name, step_type, request_config, extractors, assertions, timeout_ms, failure_policy FROM scenario_steps WHERE system_id = ? AND scenario_version_id = ? AND enabled = TRUE ORDER BY position, id`

type MySQLStepProvider struct{ db *sql.DB }

func NewMySQLStepProvider(db *sql.DB) *MySQLStepProvider { return &MySQLStepProvider{db: db} }

func (provider *MySQLStepProvider) Steps(ctx context.Context, systemID, scenarioVersionID string) ([]Step, error) {
	rows, err := provider.db.QueryContext(ctx, selectScenarioStepsSQL, systemID, scenarioVersionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	steps := make([]Step, 0)
	for rows.Next() {
		var step Step
		var requestConfig, extractors, assertions []byte
		var timeoutMS int
		var failurePolicy string
		if err := rows.Scan(&step.ID, &step.Position, &step.Name, &step.Type, &requestConfig, &extractors, &assertions, &timeoutMS, &failurePolicy); err != nil {
			return nil, err
		}
		step.Config = map[string]any{}
		if len(requestConfig) > 0 {
			if err := json.Unmarshal(requestConfig, &step.Config); err != nil {
				return nil, err
			}
		}
		if value, err := decodeAnyJSON(extractors); err != nil {
			return nil, err
		} else if value != nil {
			step.Config["extractors"] = value
		}
		if value, err := decodeAnyJSON(assertions); err != nil {
			return nil, err
		} else if value != nil {
			step.Config["assertions"] = value
		}
		step.Config["timeoutMs"] = timeoutMS
		step.Config["failurePolicy"] = failurePolicy
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func decodeAnyJSON(data []byte) (any, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

var _ StepProvider = (*MySQLStepProvider)(nil)
