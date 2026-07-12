package execution

import "context"

// DisabledExecutor is the safe production default until a connector-aware
// executor is explicitly composed. It never performs network or script work.
type DisabledExecutor struct{}

func (DisabledExecutor) Execute(context.Context, Step, map[string]any) (StepResult, error) {
	return StepResult{RequestSnapshot: map[string]any{"executor": "disabled"}}, ErrExecutorNotConfigured
}

var _ StepExecutor = DisabledExecutor{}
