# kubectl-bcm-audit / kubedrift-audit

Local **Kubernetes BCM/DORA backup coverage audit** against [Velero](https://velero.io/) schedules, backups, and restores. No telemetry, no SaaS — optional `client-go` access to your cluster only when you pass a kubeconfig.

**Module:** `github.com/sysrqio/kubectl-bcm-audit`  
**License:** Apache-2.0

## Install

```bash
go install github.com/sysrqio/kubectl-bcm-audit/cmd/kubedrift-audit@latest
# binary names: kubedrift-audit (primary), build also supports kubectl-bcm-audit symlink
```

Or build from source:

```bash
make build
./bin/kubedrift-audit version
```

## Usage

### Scan (default)

```bash
# Offline / CI with fixture snapshot
kubedrift-audit scan --fixture testdata/cluster.json

# Live cluster (requires Velero CRDs)
kubedrift-audit scan --kubeconfig ~/.kube/config --velero-namespace velero

# Root alias (same as scan)
kubedrift-audit --fixture testdata/cluster.json --output json
```

| Flag | Default | Description |
|------|---------|-------------|
| `--namespace` | all (scoped) | Single namespace |
| `--exclude-namespaces` | `kube-system,kube-public,kube-node-lease` | Skip system namespaces |
| `--velero-namespace` | `velero` | Velero install namespace |
| `--standard` | `dora` | Heuristic: `dora` or `bsi` |
| `--output` | `terminal` | `terminal`, `json`, `markdown`, `html` |
| `--fail-on-drift` | false | Exit code **2** if uncovered PVCs |
| `--kubeconfig` | `$KUBECONFIG` | Live cluster mode |
| `--fixture` | — | JSON cluster snapshot (offline) |

**Exit codes:** `0` OK · `1` kubeconfig/fixture error · `2` drift (with `--fail-on-drift`)

### Explain a PVC

```bash
kubedrift-audit explain postgres-data-postgres-0 \
  --namespace app-prod \
  --fixture testdata/cluster.json
```

### Version

```bash
kubedrift-audit version
```

## Audit logic

1. **Schedule correlation** — PVCs in scope without a matching active Velero `Schedule` → drift (`PVC_NO_BACKUP_SCHEDULE`).
2. **Backup window** — No recent successful `Backup` for covered namespaces → `BACKUP_WINDOW_GAP`.
3. **Restore evidence** — Completed backups without a recent successful `Restore` → `RESTORE_EVIDENCE_GAP` (window depends on `--standard`).
4. **Score** — `overall_score_percentage` from PVC schedule coverage minus drift penalty.

## Fixture format

See `testdata/cluster.json` for PVCs, StatefulSets, and Velero schedules/backups/restores. Use fixtures in unit tests and air-gapped pipelines with zero cluster egress.

## Krew (optional)

Example manifest: [`deploy/krew/kubectl-bcm-audit.yaml`](deploy/krew/kubectl-bcm-audit.yaml). Publish release artifacts and replace the `sha256` placeholder before indexing.

```bash
kubectl krew install bcm-audit   # after manifest is published to krew-index
```

## Development

```bash
make test
make lint
```

CI runs `go test ./...` offline (no Kubernetes API).
