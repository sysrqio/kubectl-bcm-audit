package standards

import "time"

// Standard defines BCM/DORA expectations for restore evidence windows.
type Standard string

const (
	DORA Standard = "dora"
	BSI  Standard = "bsi"
)

type Requirements struct {
	Name                      string
	RestoreEvidenceMaxAge     time.Duration
	RequireSchedulePerPVC     bool
	MinSuccessfulBackupWindow time.Duration
	Description               string
}

func Get(std Standard) Requirements {
	switch std {
	case BSI:
		return Requirements{
			Name:                      "BSI IT-Grundschutz (backup coverage heuristic)",
			RestoreEvidenceMaxAge:     90 * 24 * time.Hour,
			RequireSchedulePerPVC:     true,
			MinSuccessfulBackupWindow: 7 * 24 * time.Hour,
			Description:               "Stricter restore-test cadence and namespace-scoped schedule expectations.",
		}
	default:
		return Requirements{
			Name:                      "DORA operational resilience (backup coverage heuristic)",
			RestoreEvidenceMaxAge:     30 * 24 * time.Hour,
			RequireSchedulePerPVC:     true,
			MinSuccessfulBackupWindow: 7 * 24 * time.Hour,
			Description:               "Monthly restore evidence and weekly successful backup window.",
		}
	}
}

func Parse(s string) Standard {
	switch s {
	case "bsi", "BSI":
		return BSI
	default:
		return DORA
	}
}
