# Project: Cloud Mod Manager (cmm) Mod Enable/Disable Lifecycle System

## Architecture
Cloud Mod Manager (`cmm`) is a CLI and TUI tool for managing Minecraft modpacks via the Modrinth API.
The Mod Enable/Disable Lifecycle System introduces the ability to toggle installed mods between active (`.jar`) and disabled (`.jar.disabled`) states across the filesystem, lockfile configuration (`cmm.lock`), CLI commands, synchronization engines, and the interactive terminal UI.

### Data Flow & Component Interaction:
1. **Configuration Layer (`internal/config/`)**:
   - `cmm.lock` stores mod metadata. `LockfileMod` includes `Disabled bool` (`toml:"disabled,omitempty" json:"disabled,omitempty"`).
   - Filename normalization (`ExtractModSlug`, `matchMod`) transparently strips both `.jar` and `.jar.disabled` suffixes to map files to canonical slugs.
2. **Lifecycle Engine (`internal/mod/`)**:
   - `Manager.DisableMod`: Checks active dependents; if none or `--force`, atomically renames `mods/<file>.jar` -> `mods/<file>.jar.disabled` and updates `cmm.lock`. Graceful notice if already disabled.
   - `Manager.EnableMod`: Atomically renames `mods/<file>.jar.disabled` -> `mods/<file>.jar` and updates `cmm.lock`. Graceful notice if already enabled.
   - `Manager.DetectActiveDependents`: Traverses `cmm.lock` for active (`!Disabled`) mods requiring the target mod.
   - `Manager.List`: Populates `ModStatus.Disabled`.
3. **CLI Layer (`cmd/cmm/commands/`)**:
   - `cmm disable <slug|name>... [--force] [--dry-run]`
   - `cmm enable <slug|name>... [--dry-run]`
   - `cmm list [--format table|json]`: Displays `[DISABLED]` badge in status column and `"disabled": true/false` in JSON.
4. **Sync & Scanner Layer (`internal/sync/`)**:
   - `LocalSynchronizer`: Scans both `*.jar` and `*.jar.disabled`, computes SHA-512/SHA-1 hashes, queries Modrinth API, and records `Disabled = true` for disabled jars.
   - `DeltaEngine` & `PushSynchronizer`: Preserves `.jar.disabled` files and `Disabled` state across daemon synchronization and lockfile diffing.
5. **Interactive TUI Layer (`internal/tui/`)**:
   - Tab 1 (Installed Mods): `e` and `Space` hotkeys toggle mod enable/disable status with instant UI feedback, `[DISABLED]` styling in table rows, and `Disabled: Yes / No` in details modal.

---

## Code Layout

- `cmd/cmm/commands/`:
  - `disable.go` (new) — `cmm disable` CLI command
  - `enable.go` (new) — `cmm enable` CLI command
  - `list.go` (modified) — `cmm list` table and JSON output formatting
  - `scan.go` (modified) — `cmm scan` disabled mod reporting
  - `root.go` (modified) — register enable/disable subcommands
- `internal/config/`:
  - `lockfile.go` (modified) — `Disabled` field in `LockfileMod`, slug extraction & matching
  - `lockfile_test.go` (modified/extended) — unit tests for serialization and slug normalization
- `internal/mod/`:
  - `lifecycle.go` (new) — `DisableMod`, `EnableMod`, `ToggleMod`, active dependency safety checks
  - `manager.go` (modified) — lifecycle integration and `List()` status population
  - `dependency.go` (modified) — `DetectActiveDependents` traversal
  - `types.go` (modified) — `DisableResult`, `EnableResult`, `ModStatus.Disabled`
  - `lifecycle_test.go` (new) — unit tests for lifecycle engine
- `internal/sync/`:
  - `local.go` (modified) — dual extension scanner (`*.jar` and `*.jar.disabled`)
  - `types.go` (modified) — `TargetFile.Disabled` field
  - `engine.go` (modified) — delta engine file pruning invariance
  - `diff.go` (modified) — diff matching with `.jar.disabled`
  - `local_test.go` (modified/extended) — unit tests for local scanner
- `internal/tui/`:
  - `tabs/mods.go` (modified) — `e` and `Space` hotkey toggles, `[DISABLED]` badge & dimming, modal status
  - `components/help.go` (modified) — keybinding help documentation
- `e2e/`:
  - `tier1/` — feature coverage tests (enable, disable, list, scan, tui)
  - `tier2/` — boundary, idempotency, safety overrides, dry-run tests
  - `tier3/` — pairwise cross-feature interaction tests
  - `tier4/` — real-world modpack workload tests

---

## Feature Inventory

Every feature discovered during the Survey phase is inventoried and assigned to a milestone:

| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| 1 | Lockfile Schema Extension | Add `Disabled bool` field to `LockfileMod` (`toml:"disabled,omitempty" json:"disabled,omitempty"`) | M1 | Survey §4.1 F1 |
| 2 | Slug Extraction Normalization | Normalize `.jar.disabled` in `ExtractModSlug` and `matchMod` | M1 | Survey §4.1 F1 |
| 3 | DisableMod Operation | Renames `.jar` -> `.jar.disabled`, checks dependents, updates `cmm.lock` | M1 | Survey §4.1 F2 |
| 4 | EnableMod Operation | Renames `.jar.disabled` -> `.jar`, updates `cmm.lock` | M1 | Survey §4.1 F3 |
| 5 | Active Dependents Safety Check | `DetectActiveDependents` warns if active mods depend on disabled mod | M1 | Survey §4.1 F5 |
| 6 | Graceful Idempotency | Already enabled/disabled operations return notices with exit code 0 | M1 | Survey §4.1 F2, F3 |
| 7 | ModStatus Disabled Extension | Include `Disabled bool` in `ModStatus` and populate in `Manager.List` | M1 | Survey §4.1 F1 |
| 8 | `cmm disable` CLI Command | CLI command with `--force` and `--dry-run` flags | M2 | Survey §4.1 F6 |
| 9 | `cmm enable` CLI Command | CLI command with `--dry-run` flag | M2 | Survey §4.1 F7 |
| 10 | `cmm list` Disabled Rendering | Table shows `[DISABLED]` status; JSON outputs `"disabled": true/false` | M2 | Survey §4.1 F8 |
| 11 | Dual Extension Scanner | `cmm scan` / `cmm sync local` scans `*.jar` and `*.jar.disabled` | M3 | Survey §4.1 F9 |
| 12 | Scanner Lockfile State Persistence | Scanner populates `Disabled = true` in `cmm.lock` for `.jar.disabled` | M3 | Survey §4.1 F10 |
| 13 | Delta Sync & Diff Preservation | Remote push and diff engine preserve disabled state | M3 | Survey §4.1 F11, F12 |
| 14 | TUI Tab 1 Hotkey Toggle | `e` and `Space` toggle enable/disable status with instant UI feedback | M4 | Survey §4.1 F13 |
| 15 | TUI Disabled Badge & Dimming | `[DISABLED]` badge styling and row dimming in Tab 1 | M4 | Survey §4.1 F14 |
| 16 | TUI Details Modal State | Modal shows `Disabled: Yes / No` | M4 | Survey §4.1 F15 |
| 17 | TUI Help Dialog Update | Keybinding documentation for `e` / `Space` | M4 | Survey §4.1 F16 |
| 18 | E2E Tier 1 Feature Coverage | Opaque-box tests for enable, disable, list, scan, TUI | M5 / Test Track | Survey §4.1 F17 |
| 19 | E2E Tier 2 Boundary & Safety | Idempotency, nonexistent mods, force overrides, dry-run | M5 / Test Track | Survey §4.1 F17 |
| 20 | E2E Tier 3 Pairwise Combinations | Cross-feature interactions (enable/disable + update, pin, sync, push, diff) | M5 / Test Track | Survey §4.1 F17 |
| 21 | E2E Tier 4 Real-World Workloads | Modpack workloads with deep dependency chains and toggle cycles | M5 / Test Track | Survey §4.1 F17 |
| 22 | 5-Target Cross-Compilation | Builds static binaries for Linux, macOS, Windows (`make cross-compile`) | M5 | Survey §4.1 F18 |

---

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | Lifecycle Engine & Lockfile Extension | `internal/config/lockfile.go`, `internal/mod/lifecycle.go`, `internal/mod/manager.go`, `internal/mod/dependency.go`, `internal/mod/types.go` + unit tests | none | PLANNED |
| M2 | CLI Commands | `cmd/cmm/commands/disable.go`, `cmd/cmm/commands/enable.go`, `cmd/cmm/commands/list.go`, `cmd/cmm/commands/root.go` + CLI tests | M1 | PLANNED |
| M3 | Scanner & Sync Engine Compatibility | `internal/sync/local.go`, `internal/sync/engine.go`, `internal/sync/diff.go`, `internal/sync/types.go`, `cmd/cmm/commands/scan.go` + unit tests | M1 | PLANNED |
| M4 | TUI Integration | `internal/tui/tabs/mods.go`, `internal/tui/components/help.go` + TUI tests | M1 | PLANNED |
| M5 | E2E Integration & Build Verification | Pass 100% E2E test suites (Tiers 1-4), Tier 5 adversarial hardening, `make test-all`, `make cross-compile` | M1, M2, M3, M4, Test Track | PLANNED |

---

## Interface Contracts

### 1. Lockfile Schema (`internal/config/lockfile.go`)
```go
type LockfileMod struct {
    Slug          string `toml:"slug,omitempty" json:"slug,omitempty"`
    Name          string `toml:"name,omitempty" json:"name,omitempty"`
    Source        string `toml:"source,omitempty" json:"source,omitempty"`
    ProjectID     string `toml:"project_id,omitempty" json:"project_id,omitempty"`
    VersionID     string `toml:"version_id,omitempty" json:"version_id,omitempty"`
    VersionNumber string `toml:"version_number,omitempty" json:"version_number,omitempty"`
    Version       string `toml:"version,omitempty" json:"version,omitempty"`
    FileName      string `toml:"file_name,omitempty" json:"file_name,omitempty"`
    SHA512        string `toml:"sha512,omitempty" json:"sha512,omitempty"`
    DownloadURL   string `toml:"download_url,omitempty" json:"download_url,omitempty"`
    ClientSide    string `toml:"client_side,omitempty" json:"client_side,omitempty"`
    ServerSide    string `toml:"server_side,omitempty" json:"server_side,omitempty"`
    Side          string `toml:"side,omitempty" json:"side,omitempty"`
    Pinned        bool   `toml:"pinned" json:"pinned"`
    Disabled      bool   `toml:"disabled,omitempty" json:"disabled,omitempty"`
}

func (l *Lockfile) IsDisabled(slugOrID string) bool
func (l *Lockfile) SetDisabled(slugOrID string, disabled bool) bool
```

### 2. Mod Lifecycle & Status (`internal/mod/lifecycle.go`, `internal/mod/types.go`)
```go
type DisableOptions struct {
    Force  bool
    DryRun bool
}

type EnableOptions struct {
    DryRun bool
}

type DisableResult struct {
    Slug                 string   `json:"slug"`
    Name                 string   `json:"name"`
    OldFileName          string   `json:"old_file_name"`
    NewFileName          string   `json:"new_file_name"`
    AlreadyDisabled      bool     `json:"already_disabled"`
    DryRun               bool     `json:"dry_run"`
    HasDependentsWarning bool     `json:"has_dependents_warning"`
    ActiveDependents     []string `json:"active_dependents,omitempty"`
}

type EnableResult struct {
    Slug           string `json:"slug"`
    Name           string `json:"name"`
    OldFileName    string `json:"old_file_name"`
    NewFileName    string `json:"new_file_name"`
    AlreadyEnabled bool   `json:"already_enabled"`
    DryRun         bool   `json:"dry_run"`
}

type ModStatus struct {
    Name            string `json:"name"`
    Slug            string `json:"slug"`
    Version         string `json:"version"`
    Side            string `json:"side"`
    Pinned          bool   `json:"pinned"`
    Disabled        bool   `json:"disabled"`
    UpdateAvailable bool   `json:"update_available"`
    LatestVersion   string `json:"latest_version,omitempty"`
}

func (m *Manager) DisableMod(slugOrID string, opts DisableOptions) (*DisableResult, error)
func (m *Manager) EnableMod(slugOrID string, opts EnableOptions) (*EnableResult, error)
func (m *Manager) DetectActiveDependents(slugOrID string, lock *config.Lockfile) ([]string, error)
```

### 3. Sync & Target File (`internal/sync/types.go`)
```go
type TargetFile struct {
    FileName    string
    SHA512      string
    DownloadURL string
    Slug        string
    Name        string
    ProjectID   string
    VersionID   string
    Version     string
    Side        string
    Disabled    bool
}
```
