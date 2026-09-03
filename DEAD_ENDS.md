# Dead Ends Log

| Iteration | Approach Tried | Why It Failed | Files Touched |
|---|---|---|---|
| M1 Iter 1 | Single-pass combined exact & normalized lookup in `matchMod` | `ExtractModSlug` stripped trailing digits from numbered slugs (`mod-01` -> `mod`), causing single-pass search to match `mod-01` when looking up `mod-02`. Also unconditional `filepath.Ext` on `.disabled` stripped version components (`.21`). | `internal/config/lockfile.go` |
| M1 Iter 2 | Two-pass lookup where Pass 2 normalized fallback compared `ExtractModSlug(mod.FileName) == ExtractModSlug(query)` in `AddOrUpdateMod` and `GetMod` | `AddOrUpdateMod` used Pass 2 fallback, causing additions of new numbered mods (`chain-mod-01`) to overwrite existing mod at index 0 (`chain-mod-00`). Also negative queries (`GetMod("mod-99")`) matched `mod-01`. Also prefix loader guards prevented secondary loader stripping for `forge-config-api-port+forge...`. | `internal/config/lockfile.go`, `internal/mod/challenger_m1_test.go` |
