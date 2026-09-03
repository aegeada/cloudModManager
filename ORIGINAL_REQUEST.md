# Original User Request

## 2026-09-01T22:07:04Z

Build the **Mod Enable/Disable Lifecycle System** for "Cloud Mod Manager" (`cmm`) — allowing users to toggle mods between active (`.jar`) and disabled (`.jar.disabled`) states via CLI (`cmm enable`, `cmm disable`) and TUI (hotkey toggle), tracking state in `cmm.lock`, checking dependent mod safety warnings, and seamlessly integrating with local scanners and remote sync engines.

Working directory: /home/lunarea/Documents/workspace/cloudModManager
Integrity mode: demo

## Reference Material & Completed Base (Milestones 1–5)

- **Existing & Working Components**:
  - Full CLI command suite: `init`, `search`, `add`, `remove`, `list`, `pin`, `unpin`, `update`, `loader`, `sync`, `serve`, `export`, `tui`, `scan`, `self-install`, `push`, `diff`, `launcher`.
  - `internal/config/`: `cmm.toml` and `cmm.lock` manager.
  - `internal/mod/`: `Manager` handling downloads, dependency resolution, lockfile updates.
  - `internal/sync/`: Scanner (`local.go`), Modrinth delta engine, server daemon.
  - `internal/tui/`: Bubbletea 4-tab user interface (`mods.go`, `search.go`, `config.go`, `sync.go`).
  - All unit, integration, and adversarial tests passing 100%. Cross-compiled static binaries in `bin/`.
  - Go runtime: `./go/bin/go` (export PATH="/home/lunarea/Documents/workspace/cloudModManager/go/bin:$PATH")

## Requirements

### R1. Enable/Disable Lifecycle Engine (`internal/mod/lifecycle.go` & `internal/config/lockfile.go`)
- **Lockfile Schema Extension**:
  - Add `Disabled bool` field to `LockfileMod` struct in `internal/config/lockfile.go` (`toml:"disabled,omitempty" json:"disabled,omitempty"`).
- **Disable Operation (`DisableMod`)**:
  - Find mod by slug, name, or filename in `cmm.lock` and `mods/`.
  - Check if other active/enabled mods depend on this mod; warn if dependencies are found (allow bypass with `--force`).
  - Atomically rename `mods/<filename>.jar` to `mods/<filename>.jar.disabled` on disk.
  - Update `cmm.lock` entry: set `Disabled = true` and update `FileName = "<filename>.jar.disabled"`.
- **Enable Operation (`EnableMod`)**:
  - Find mod in `cmm.lock` and `mods/`.
  - Atomically rename `mods/<filename>.jar.disabled` back to `mods/<filename>.jar` on disk.
  - Update `cmm.lock` entry: set `Disabled = false` and update `FileName = "<filename>.jar"`.

### R2. CLI Commands (`cmd/cmm/commands/enable.go`, `disable.go`, `list.go`)
- **`cmm disable <slug|name> [--force] [--dry-run]`**:
  - Disables the target mod(s), renames JAR files, and updates lockfile.
- **`cmm enable <slug|name> [--dry-run]`**:
  - Re-enables disabled mod(s), restores `.jar` extension, and updates lockfile.
- **`cmm list`**:
  - Display mod state in the list table: show `[DISABLED]` in status or display an `ENABLED` / `DISABLED` indicator.

### R3. Scanner & Sync Engine Compatibility (`internal/sync/local.go`, `cmd/cmm/commands/scan.go`, `internal/sync/diff.go`)
- **Local Scanner (`cmm scan` / `cmm sync local`)**:
  - Scan both `*.jar` and `*.jar.disabled` files in `mods/`.
  - Extract hashes and query Modrinth `/v2/version_files` for both active and disabled jars.
  - Record `Disabled = true` for `.jar.disabled` files in `cmm.lock` without removing or altering them.
- **Remote Push & Diff**:
  - Preserve `disabled` status across client-to-server push and lockfile diff comparisons.

### R4. TUI Integration (`internal/tui/tabs/mods.go`)
- In Tab 1 (Installed Mods):
  - Add keybinding `e` (and `Space`) to toggle mod enable/disable status with instant UI feedback.
  - Highlight disabled mods with distinct badge `[DISABLED]` or dimmed styling in the table.
  - Show `Disabled: Yes / No` in the Mod Details modal (`Enter`).

## Acceptance Criteria

### Enable/Disable Operations
- [ ] `cmm disable <mod>` renames file to `.jar.disabled` and marks `disabled = true` in `cmm.lock`.
- [ ] `cmm enable <mod>` renames file back to `.jar` and marks `disabled = false` in `cmm.lock`.
- [ ] Attempting to disable an already disabled mod or enable an already active mod outputs a clear, graceful notice without error.
- [ ] Dependency warning is displayed when disabling a mod with active dependents unless `--force` is used.

### Scanner & Sync Compatibility
- [ ] `cmm scan` and `cmm sync local` correctly detect `.jar.disabled` files and mark them as disabled in `cmm.lock`.
- [ ] `cmm list` cleanly presents disabled state.
- [ ] TUI `e` / `Space` hotkey toggles mod status interactively.

### Quality & Build Verification
- [ ] `make test-unit` passes 100% across all packages.
- [ ] `make test-e2e` passes 100% of integration test tiers (Tier 1–4).
- [ ] `make cross-compile` builds all 5 static binaries in `bin/`.
