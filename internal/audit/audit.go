package audit

import (
	"fmt"
	"strings"
	"time"

	"github.com/sysrqio/kubectl-bcm-audit/internal/cluster"
	"github.com/sysrqio/kubectl-bcm-audit/internal/standards"
)

type FindingSeverity string

const (
	SeverityCritical FindingSeverity = "critical"
	SeverityWarning  FindingSeverity = "warning"
	SeverityInfo     FindingSeverity = "info"
)

type Finding struct {
	Code        string          `json:"code"`
	Severity    FindingSeverity `json:"severity"`
	Namespace   string          `json:"namespace,omitempty"`
	Resource    string          `json:"resource,omitempty"`
	Message     string          `json:"message"`
	Remediation string          `json:"remediation,omitempty"`
}

type PVCDetail struct {
	Namespace       string   `json:"namespace"`
	Name            string   `json:"name"`
	HasSchedule     bool     `json:"has_schedule"`
	MatchedSchedules []string `json:"matched_schedules,omitempty"`
	RecentBackup    string   `json:"recent_backup,omitempty"`
	RestoreEvidence bool     `json:"restore_evidence"`
	Findings        []Finding `json:"findings,omitempty"`
}

type Report struct {
	Standard                string    `json:"standard"`
	StandardDescription     string    `json:"standard_description"`
	GeneratedAt             time.Time `json:"generated_at"`
	OverallScorePercentage  float64   `json:"overall_score_percentage"`
	TotalPVCs               int       `json:"total_pvcs"`
	CoveredPVCs             int       `json:"covered_pvcs"`
	DriftCount              int       `json:"drift_count"`
	RestoreEvidenceGapCount int       `json:"restore_evidence_gap_count"`
	Findings                []Finding `json:"findings"`
	PVCIndex                map[string]PVCDetail `json:"pvc_index,omitempty"`
	HasDrift                bool      `json:"has_drift"`
}

type Options struct {
	Filter      cluster.FilterOptions
	VeleroNS    string
	Standard    standards.Standard
	Now         time.Time
}

func Run(snap *cluster.Snapshot, opt Options) *Report {
	if opt.Now.IsZero() {
		opt.Now = time.Now().UTC()
	}
	req := standards.Get(opt.Standard)

	activeSchedules := filterActiveSchedules(snap.Velero.Schedules)
	scheduleByNS := buildScheduleCoverage(activeSchedules)

	var findings []Finding
	covered := 0
	total := 0
	pvcIndex := make(map[string]PVCDetail)

	for _, pvc := range snap.PVCs {
		if !opt.Filter.Allows(pvc.Namespace) {
			continue
		}
		total++
		key := pvc.Namespace + "/" + pvc.Name
		detail := PVCDetail{
			Namespace: pvc.Namespace,
			Name:      pvc.Name,
		}

		matched := schedulesForPVC(pvc, scheduleByNS, activeSchedules)
		if len(matched) > 0 {
			detail.HasSchedule = true
			detail.MatchedSchedules = matched
			covered++
		} else if req.RequireSchedulePerPVC {
			f := Finding{
				Code:        "PVC_NO_BACKUP_SCHEDULE",
				Severity:    SeverityCritical,
				Namespace:   pvc.Namespace,
				Resource:    "PersistentVolumeClaim/" + pvc.Name,
				Message:     fmt.Sprintf("PVC %s/%s has no matching Velero backup schedule", pvc.Namespace, pvc.Name),
				Remediation: "Create or extend a Velero Schedule that includes this namespace (and PVC resources).",
			}
			findings = append(findings, f)
			detail.Findings = append(detail.Findings, f)
		}

		recentBackup := latestCompletedBackupForNS(snap.Velero.Backups, pvc.Namespace, matched, opt.Now, req.MinSuccessfulBackupWindow)
		if recentBackup != "" {
			detail.RecentBackup = recentBackup
		} else if detail.HasSchedule {
			f := Finding{
				Code:        "BACKUP_WINDOW_GAP",
				Severity:    SeverityWarning,
				Namespace:   pvc.Namespace,
				Resource:    "PersistentVolumeClaim/" + pvc.Name,
				Message:     fmt.Sprintf("No successful Velero backup within %s for namespace %s", req.MinSuccessfulBackupWindow, pvc.Namespace),
				Remediation: "Verify Velero schedule cron and backup storage location.",
			}
			findings = append(findings, f)
			detail.Findings = append(detail.Findings, f)
		}

		if detail.RecentBackup != "" && !hasRecentRestore(snap.Velero.Restores, detail.RecentBackup, opt.Now, req.RestoreEvidenceMaxAge) {
			f := Finding{
				Code:        "RESTORE_EVIDENCE_GAP",
				Severity:    SeverityWarning,
				Namespace:   pvc.Namespace,
				Resource:    "PersistentVolumeClaim/" + pvc.Name,
				Message:     fmt.Sprintf("Backup %s lacks recent successful restore evidence (within %s)", detail.RecentBackup, req.RestoreEvidenceMaxAge),
				Remediation: "Run periodic restore drills and document Velero Restore objects.",
			}
			findings = append(findings, f)
			detail.Findings = append(detail.Findings, f)
		} else if detail.RecentBackup != "" {
			detail.RestoreEvidence = true
		}

		pvcIndex[key] = detail
	}

	driftCount := countDrift(findings)
	restoreGaps := countCode(findings, "RESTORE_EVIDENCE_GAP")
	score := 100.0
	if total > 0 {
		score = float64(covered) / float64(total) * 100
		penalty := float64(driftCount) * 5
		if penalty > 40 {
			penalty = 40
		}
		score -= penalty
		if score < 0 {
			score = 0
		}
	}

	return &Report{
		Standard:                string(opt.Standard),
		StandardDescription:     req.Description,
		GeneratedAt:             opt.Now,
		OverallScorePercentage:  score,
		TotalPVCs:               total,
		CoveredPVCs:             covered,
		DriftCount:              driftCount,
		RestoreEvidenceGapCount: restoreGaps,
		Findings:                findings,
		PVCIndex:                pvcIndex,
		HasDrift:                driftCount > 0,
	}
}

func Explain(snap *cluster.Snapshot, opt Options, pvcName string) (*PVCDetail, error) {
	rep := Run(snap, opt)
	var ns string
	name := pvcName
	if strings.Contains(pvcName, "/") {
		parts := strings.SplitN(pvcName, "/", 2)
		ns, name = parts[0], parts[1]
	} else if opt.Filter.Namespace != "" {
		ns = opt.Filter.Namespace
	} else {
		for key, d := range rep.PVCIndex {
			if strings.HasSuffix(key, "/"+pvcName) {
				return copyDetail(d), nil
			}
		}
		return nil, fmt.Errorf("PVC %q not found in scoped audit (use namespace/name)", pvcName)
	}
	key := ns + "/" + name
	d, ok := rep.PVCIndex[key]
	if !ok {
		return nil, fmt.Errorf("PVC %s/%s not in audit scope or does not exist in snapshot", ns, name)
	}
	return copyDetail(d), nil
}

func copyDetail(d PVCDetail) *PVCDetail {
	cp := d
	if len(d.Findings) > 0 {
		cp.Findings = append([]Finding(nil), d.Findings...)
	}
	if len(d.MatchedSchedules) > 0 {
		cp.MatchedSchedules = append([]string(nil), d.MatchedSchedules...)
	}
	return &cp
}

func filterActiveSchedules(scheds []cluster.VeleroSchedule) []cluster.VeleroSchedule {
	var out []cluster.VeleroSchedule
	for _, s := range scheds {
		if !s.Paused {
			out = append(out, s)
		}
	}
	return out
}

func buildScheduleCoverage(scheds []cluster.VeleroSchedule) map[string][]string {
	m := make(map[string][]string)
	for _, s := range scheds {
		if len(s.IncludedNamespaces) == 0 {
			m["*"] = append(m["*"], s.Name)
			continue
		}
		for _, ns := range s.IncludedNamespaces {
			m[ns] = append(m[ns], s.Name)
		}
	}
	return m
}

func schedulesForPVC(pvc cluster.PVC, byNS map[string][]string, scheds []cluster.VeleroSchedule) []string {
	var names []string
	seen := make(map[string]struct{})
	add := func(list []string) {
		for _, n := range list {
			if _, ok := seen[n]; !ok {
				seen[n] = struct{}{}
				names = append(names, n)
			}
		}
	}
	add(byNS[pvc.Namespace])
	add(byNS["*"])
	if len(names) == 0 {
		for _, s := range scheds {
			if len(s.IncludedNamespaces) == 0 {
				add([]string{s.Name})
			}
		}
	}
	return names
}

func latestCompletedBackupForNS(backups []cluster.VeleroBackup, ns string, schedules []string, now time.Time, window time.Duration) string {
	cutoff := now.Add(-window)
	var best cluster.VeleroBackup
	found := false
	for _, b := range backups {
		if !strings.EqualFold(b.Status, "Completed") {
			continue
		}
		t := b.CompletedAt
		if t.IsZero() {
			t = b.StartedAt
		}
		if t.Before(cutoff) {
			continue
		}
		if b.Schedule != "" && len(schedules) > 0 {
			match := false
			for _, s := range schedules {
				if s == b.Schedule {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		if !found || t.After(best.CompletedAt) {
			best = b
			found = true
		}
	}
	if !found {
		// fallback: any completed backup in window (namespace correlation is weak in Velero backup objects)
		for _, b := range backups {
			if !strings.EqualFold(b.Status, "Completed") {
				continue
			}
			t := b.CompletedAt
			if t.IsZero() {
				t = b.StartedAt
			}
			if t.Before(cutoff) {
				continue
			}
			if !found || t.After(best.CompletedAt) {
				best = b
				found = true
			}
		}
	}
	if found {
		return best.Name
	}
	return ""
}

func hasRecentRestore(restores []cluster.VeleroRestore, backupName string, now time.Time, maxAge time.Duration) bool {
	cutoff := now.Add(-maxAge)
	for _, r := range restores {
		if r.BackupName != backupName {
			continue
		}
		if !strings.EqualFold(r.Status, "Completed") {
			continue
		}
		t := r.CompletedAt
		if t.IsZero() {
			continue
		}
		if !t.Before(cutoff) {
			return true
		}
	}
	return false
}

func countDrift(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if f.Code == "PVC_NO_BACKUP_SCHEDULE" {
			n++
		}
	}
	return n
}

func countCode(findings []Finding, code string) int {
	n := 0
	for _, f := range findings {
		if f.Code == code {
			n++
		}
	}
	return n
}
