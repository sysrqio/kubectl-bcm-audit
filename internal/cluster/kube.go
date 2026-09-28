package cluster

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// LoadFromCluster builds a snapshot using client-go (optional live path).
func LoadFromCluster(ctx context.Context, kubeconfig, veleroNS string) (*Snapshot, error) {
	kc := kubeconfig
	if kc == "" {
		if home := homedir.HomeDir(); home != "" {
			kc = filepath.Join(home, ".kube", "config")
		}
	}
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kc
	configOverrides := &clientcmd.ConfigOverrides{}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	snap := &Snapshot{Velero: VeleroResources{Namespace: veleroNS}}

	nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	for _, ns := range nsList.Items {
		snap.Namespaces = append(snap.Namespaces, ns.Name)
	}

	pvcList, err := clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pvcs: %w", err)
	}
	for _, pvc := range pvcList.Items {
		size := ""
		if req, ok := pvc.Spec.Resources.Requests["storage"]; ok {
			size = req.String()
		}
		snap.PVCs = append(snap.PVCs, PVC{
			Namespace:    pvc.Namespace,
			Name:         pvc.Name,
			Labels:       pvc.Labels,
			StorageClass: ptrStr(pvc.Spec.StorageClassName),
			Size:         size,
		})
	}

	ssList, err := clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list statefulsets: %w", err)
	}
	for _, ss := range ssList.Items {
		var pvcs []string
		for _, tpl := range ss.Spec.VolumeClaimTemplates {
			pvcs = append(pvcs, tpl.Name)
		}
		snap.StatefulSets = append(snap.StatefulSets, StatefulSet{
			Namespace: ss.Namespace,
			Name:      ss.Name,
			Labels:    ss.Labels,
			PVCNames:  pvcs,
		})
	}

	depList, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	for _, d := range depList.Items {
		snap.Deployments = append(snap.Deployments, Deployment{
			Namespace: d.Namespace,
			Name:      d.Name,
			Labels:    d.Labels,
		})
	}

	if err := loadVeleroCRDs(ctx, dyn, veleroNS, snap); err != nil {
		return nil, err
	}

	linkPVCUsage(snap)
	return snap, nil
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

var (
	veleroScheduleGVR = schema.GroupVersionResource{Group: "velero.io", Version: "v1", Resource: "schedules"}
	veleroBackupGVR   = schema.GroupVersionResource{Group: "velero.io", Version: "v1", Resource: "backups"}
	veleroRestoreGVR  = schema.GroupVersionResource{Group: "velero.io", Version: "v1", Resource: "restores"}
)

func loadVeleroCRDs(ctx context.Context, dyn dynamic.Interface, veleroNS string, snap *Snapshot) error {
	schedRI := dyn.Resource(veleroScheduleGVR).Namespace(veleroNS)
	scheds, err := schedRI.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list velero schedules: %w", err)
	}
	for _, item := range scheds.Items {
		snap.Velero.Schedules = append(snap.Velero.Schedules, unstructuredToSchedule(item))
	}

	bakRI := dyn.Resource(veleroBackupGVR).Namespace(veleroNS)
	baks, err := bakRI.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list velero backups: %w", err)
	}
	for _, item := range baks.Items {
		snap.Velero.Backups = append(snap.Velero.Backups, unstructuredToBackup(item))
	}

	resRI := dyn.Resource(veleroRestoreGVR).Namespace(veleroNS)
	ress, err := resRI.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list velero restores: %w", err)
	}
	for _, item := range ress.Items {
		snap.Velero.Restores = append(snap.Velero.Restores, unstructuredToRestore(item))
	}
	return nil
}

func unstructuredToSchedule(u unstructured.Unstructured) VeleroSchedule {
	incNS, _, _ := unstructured.NestedStringSlice(u.Object, "spec", "template", "includedNamespaces")
	paused, _, _ := unstructured.NestedBool(u.Object, "spec", "paused")
	ttl, _, _ := unstructured.NestedString(u.Object, "spec", "template", "ttl")
	return VeleroSchedule{
		Name:               u.GetName(),
		IncludedNamespaces: incNS,
		Paused:             paused,
		TTL:                ttl,
	}
}

func unstructuredToBackup(u unstructured.Unstructured) VeleroBackup {
	phase, _, _ := unstructured.NestedString(u.Object, "status", "phase")
	sched, _, _ := unstructured.NestedString(u.Object, "spec", "scheduleName")
	started, _, _ := unstructured.NestedString(u.Object, "status", "startTimestamp")
	completed, _, _ := unstructured.NestedString(u.Object, "status", "completionTimestamp")
	return VeleroBackup{
		Name:        u.GetName(),
		Schedule:    sched,
		Status:      phase,
		StartedAt:   parseTime(started),
		CompletedAt: parseTime(completed),
	}
}

func unstructuredToRestore(u unstructured.Unstructured) VeleroRestore {
	phase, _, _ := unstructured.NestedString(u.Object, "status", "phase")
	backup, _, _ := unstructured.NestedString(u.Object, "spec", "backupName")
	completed, _, _ := unstructured.NestedString(u.Object, "status", "completionTimestamp")
	return VeleroRestore{
		Name:        u.GetName(),
		BackupName:  backup,
		Status:      phase,
		CompletedAt: parseTime(completed),
	}
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func linkPVCUsage(snap *Snapshot) {
	pvcIndex := make(map[string]int)
	for i, p := range snap.PVCs {
		key := p.Namespace + "/" + p.Name
		pvcIndex[key] = i
	}
	for _, ss := range snap.StatefulSets {
		for _, tpl := range ss.PVCNames {
			key := ss.Namespace + "/" + tpl
			if idx, ok := pvcIndex[key]; ok {
				snap.PVCs[idx].UsedBy = append(snap.PVCs[idx].UsedBy, WorkloadRef{Kind: "StatefulSet", Name: ss.Name})
			}
		}
	}
}
