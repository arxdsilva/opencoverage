package domain

import "time"

type E2ERunStatus string

const (
	E2ERunStatusPassed E2ERunStatus = "passed"
	E2ERunStatusFailed E2ERunStatus = "failed"
)

type E2ESpecState string

const (
	E2ESpecStatePassed  E2ESpecState = "passed"
	E2ESpecStateFailed  E2ESpecState = "failed"
	E2ESpecStateSkipped E2ESpecState = "skipped"
	E2ESpecStatePending E2ESpecState = "pending"
	E2ESpecStateFlaky   E2ESpecState = "flaky"
)

type E2ETestRun struct {
	ID               string
	ProjectID        string
	Branch           string
	CommitSHA        string
	Author           string
	TriggerType      string
	RunTimestamp     time.Time
	FrameworkVersion string
	TestFramework    string
	PlatformType     string
	SuiteDescription string
	SuitePath        string
	TotalSpecs       int
	PassedSpecs      int
	FailedSpecs      int
	SkippedSpecs     int
	FlakedSpecs      int
	PendingSpecs     int
	Interrupted      bool
	TimedOut         bool
	DurationMS       int64
	Status           E2ERunStatus
	Environment      *string
	CreatedAt        time.Time
}

type E2ESpecResult struct {
	ID                  string
	E2ETestRunID        string
	SpecPath            string
	LeafNodeText        string
	State               E2ESpecState
	DurationMS          int64
	FailureMessage      *string
	FailureLocationFile *string
	FailureLocationLine *int
}

func EvaluateE2ERunStatus(failedSpecs int, interrupted bool, timedOut bool) E2ERunStatus {
	if failedSpecs == 0 && !interrupted && !timedOut {
		return E2ERunStatusPassed
	}
	return E2ERunStatusFailed
}
