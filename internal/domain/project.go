package domain

import "time"

type Project struct {
	ID                     string
	ProjectKey             string
	Name                   string
	Group                  *string
	DefaultBranch          string
	GlobalThresholdPercent float64
	CreatedAt              time.Time
	UpdatedAt              time.Time
}
