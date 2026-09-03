# E2E Test Infra: Cloud Mod Manager (cmm) Mod Enable/Disable Lifecycle System

## Test Philosophy
- Opaque-box, requirement-driven. No dependency on implementation design.
- Systematic 4-tier methodology: Category-Partition + BVA + Pairwise + Workload Testing.

## Feature Inventory & Test Mapping
| # | Feature | Source | Tier 1 | Tier 2 | Tier 3 | Tier 4 |
|---|---------|--------|:------:|:------:|:------:|:------:|
| 1 | Lockfile Disabled Schema & State | ORIGINAL_REQUEST §R1 | ✓ | ✓ | ✓ | ✓ |
| 2 | Mod Disable Operation (`.jar` -> `.jar.disabled`) | ORIGINAL_REQUEST §R1 | ✓ | ✓ | ✓ | ✓ |
| 3 | Mod Enable Operation (`.jar.disabled` -> `.jar`) | ORIGINAL_REQUEST §R1 | ✓ | ✓ | ✓ | ✓ |
| 4 | Active Dependents Safety & Force Override | ORIGINAL_REQUEST §R1 | ✓ | ✓ | ✓ | ✓ |
| 5 | Graceful Notice for Idempotent Enable/Disable | ORIGINAL_REQUEST §Acceptance | ✓ | ✓ | ✓ | ✓ |
| 6 | `cmm disable` CLI Command & Dry-Run | ORIGINAL_REQUEST §R2 | ✓ | ✓ | ✓ | ✓ |
| 7 | `cmm enable` CLI Command & Dry-Run | ORIGINAL_REQUEST §R2 | ✓ | ✓ | ✓ | ✓ |
| 8 | `cmm list` Table & JSON Disabled Display | ORIGINAL_REQUEST §R2 | ✓ | ✓ | ✓ | ✓ |
| 9 | `cmm scan` / `sync local` Dual Extension Discovery | ORIGINAL_REQUEST §R3 | ✓ | ✓ | ✓ | ✓ |
| 10 | Scanner Lockfile State Persistence | ORIGINAL_REQUEST §R3 | ✓ | ✓ | ✓ | ✓ |
| 11 | Push & Diff Synchronization Preservation | ORIGINAL_REQUEST §R3 | ✓ | ✓ | ✓ | ✓ |
| 12 | TUI Hotkeys (`e`, `Space`), Badges & Modals | ORIGINAL_REQUEST §R4 | ✓ | ✓ | ✓ | ✓ |

## Test Architecture
- **Harness**: `e2e/harness/` provides `TestContext`, isolated `t.TempDir()`, Mock Modrinth API server, and CLI execution helpers.
- **Runners**:
  - Tier 1: `make test-e2e` (`go test -v ./e2e/tier1/...`)
  - Tier 2: `make test-tier2` (`go test -v ./e2e/tier2/...`)
  - Tier 3: `make test-tier3` (`go test -v ./e2e/tier3/...`)
  - Tier 4: `make test-tier4` (`go test -v ./e2e/tier4/...`)
  - All Tiers: `make test-all`

## Real-World Application Scenarios (Tier 4)
| # | Scenario | Features Exercised | Complexity |
|---|----------|--------------------|------------|
| 1 | Large Modpack Staged Disablement & Recovery | F1, F2, F3, F4, F5, F8, F9, F10 | High |
| 2 | Dependency Tree Disablement Cascade & Force | F2, F4, F5, F6, F7 | High |
| 3 | Client-Server Push & Sync with Disabled Mods | F1, F2, F9, F10, F11 | High |
| 4 | Offline Mod Scanning with Mixed Active/Disabled JARs | F1, F9, F10, F8 | Medium |
| 5 | Full Interactive Lifecycle (Add -> Disable -> Scan -> Sync -> Enable -> List) | F1-F12 | High |

## Coverage Thresholds
- Tier 1: ≥5 test cases per feature area (happy path opaque-box execution)
- Tier 2: ≥5 boundary, safety override, idempotency, dry-run, and invalid input test cases
- Tier 3: Pairwise combination tests (disable + update, pin, sync, diff, push, export)
- Tier 4: Realistic end-to-end modpack workflows
