package scanner

import "fmt"

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Job struct {
	Status Status `json:"status"`
}

func NewJob() Job { return Job{Status: StatusQueued} }

func CanTransition(from, to Status) bool {
	switch from {
	case StatusQueued:
		return to == StatusRunning
	case StatusRunning:
		return to == StatusSucceeded || to == StatusFailed
	default:
		return false
	}
}

func (job *Job) Transition(to Status) error {
	if !CanTransition(job.Status, to) {
		return fmt.Errorf("invalid scan status transition: %s -> %s", job.Status, to)
	}
	job.Status = to
	return nil
}
