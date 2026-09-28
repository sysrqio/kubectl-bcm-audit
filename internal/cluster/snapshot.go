package cluster

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Snapshot is a portable cluster state for offline audit.
type Snapshot struct {
	Namespaces []string          `json:"namespaces"`
	PVCs       []PVC             `json:"pvcs"`
	StatefulSets []StatefulSet   `json:"statefulsets"`
	Deployments  []Deployment    `json:"deployments"`
	Velero       VeleroResources `json:"velero"`
}

type PVC struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels,omitempty"`
	StorageClass string         `json:"storageClass,omitempty"`
	Size      string            `json:"size,omitempty"`
	UsedBy    []WorkloadRef     `json:"usedBy,omitempty"`
}

type WorkloadRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type StatefulSet struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels,omitempty"`
	PVCNames  []string          `json:"pvcNames,omitempty"`
}

type Deployment struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels,omitempty"`
	PVCNames  []string          `json:"pvcNames,omitempty"`
}

type VeleroResources struct {
	Namespace string            `json:"namespace"`
	Schedules []VeleroSchedule  `json:"schedules"`
	Backups   []VeleroBackup    `json:"backups"`
	Restores  []VeleroRestore   `json:"restores"`
}

type VeleroSchedule struct {
	Name              string   `json:"name"`
	IncludedNamespaces []string `json:"includedNamespaces,omitempty"`
	IncludedResources []string `json:"includedResources,omitempty"`
	LabelSelector     string   `json:"labelSelector,omitempty"`
	Paused            bool     `json:"paused"`
	TTL               string   `json:"ttl,omitempty"`
}

type VeleroBackup struct {
	Name       string    `json:"name"`
	Schedule   string    `json:"schedule,omitempty"`
	Status     string    `json:"status"` // Completed, Failed, InProgress, ...
	StartedAt  time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
}

type VeleroRestore struct {
	Name        string    `json:"name"`
	BackupName  string    `json:"backupName"`
	Status      string    `json:"status"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
}

func LoadSnapshotFromFile(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parse fixture: %w", err)
	}
	return &snap, nil
}

// FilterOptions for namespace scoping.
type FilterOptions struct {
	Namespace         string
	ExcludeNamespaces []string
}

func (f FilterOptions) Allows(ns string) bool {
	for _, ex := range f.ExcludeNamespaces {
		if ex == ns {
			return false
		}
	}
	if f.Namespace != "" && f.Namespace != ns {
		return false
	}
	return true
}

func ParseExcludeList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
