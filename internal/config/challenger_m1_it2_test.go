package config

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
)

// TestChallengerIt2_NumberedSlugDisambiguation_Exhaustive tests numbered slugs
// across extensive sets (mod-01 through mod-50, worldedit-cui-2/3, jei-1/2, etc.)
// when ALL mods are pre-populated in the lockfile.
func TestChallengerIt2_NumberedSlugDisambiguation_Exhaustive(t *testing.T) {
	const modCount = 50
	lock := &Lockfile{
		Mods: make([]LockfileMod, 0, modCount+6),
	}

	// Add mod-01 through mod-50
	for i := 1; i <= modCount; i++ {
		slug := fmt.Sprintf("mod-%02d", i)
		lock.Mods = append(lock.Mods, LockfileMod{
			Slug:          slug,
			Name:          fmt.Sprintf("Mod %02d", i),
			ProjectID:     fmt.Sprintf("proj-%02d", i),
			VersionNumber: "1.0.0",
			FileName:      fmt.Sprintf("mod-%02d-1.0.0.jar", i),
			Disabled:      false,
			Pinned:        false,
		})
	}

	// Add special pairs: worldedit-cui-2 vs worldedit-cui-3, jei-1 vs jei-2
	specialMods := []LockfileMod{
		{Slug: "worldedit-cui-2", Name: "WorldEdit CUI 2", ProjectID: "we-2", FileName: "worldedit-cui-2-fabric-1.21.jar"},
		{Slug: "worldedit-cui-3", Name: "WorldEdit CUI 3", ProjectID: "we-3", FileName: "worldedit-cui-3-fabric-1.21.jar"},
		{Slug: "jei-1", Name: "JEI 1", ProjectID: "jei-1", FileName: "jei-1-fabric-1.21.jar"},
		{Slug: "jei-2", Name: "JEI 2", ProjectID: "jei-2", FileName: "jei-2-fabric-1.21.jar"},
		{Slug: "optifine-1", Name: "OptiFine 1", ProjectID: "opt-1", FileName: "optifine-1-hd-u-i1.jar"},
		{Slug: "optifine-2", Name: "OptiFine 2", ProjectID: "opt-2", FileName: "optifine-2-hd-u-i2.jar"},
	}
	lock.Mods = append(lock.Mods, specialMods...)

	// 1. Verify GetMod by exact slug
	for i := 1; i <= modCount; i++ {
		query := fmt.Sprintf("mod-%02d", i)
		mod := lock.GetMod(query)
		if mod == nil {
			t.Fatalf("GetMod(%q) returned nil", query)
		}
		if mod.Slug != query {
			t.Fatalf("GetMod(%q) collision: returned %q instead of %q", query, mod.Slug, query)
		}
	}

	// 2. Verify special mod pairs by slug, project ID, and filename
	for _, sm := range specialMods {
		// By slug
		m := lock.GetMod(sm.Slug)
		if m == nil || m.Slug != sm.Slug {
			t.Fatalf("GetMod(%q) returned %+v, expected slug %q", sm.Slug, m, sm.Slug)
		}
		// By project ID
		m = lock.GetMod(sm.ProjectID)
		if m == nil || m.Slug != sm.Slug {
			t.Fatalf("GetMod(ProjectID %q) returned %+v, expected slug %q", sm.ProjectID, m, sm.Slug)
		}
		// By filename
		m = lock.GetMod(sm.FileName)
		if m == nil || m.Slug != sm.Slug {
			t.Fatalf("GetMod(FileName %q) returned %+v, expected slug %q", sm.FileName, m, sm.Slug)
		}
		// By filename with .disabled suffix
		disabledFileName := sm.FileName + ".disabled"
		m = lock.GetMod(disabledFileName)
		if m == nil || m.Slug != sm.Slug {
			t.Fatalf("GetMod(DisabledFileName %q) returned %+v, expected slug %q", disabledFileName, m, sm.Slug)
		}
	}

	// 3. Test SetDisabled isolated modifications
	for i := 1; i <= modCount; i++ {
		targetSlug := fmt.Sprintf("mod-%02d", i)
		if !lock.SetDisabled(targetSlug, true) {
			t.Fatalf("SetDisabled(%q, true) failed", targetSlug)
		}
		if !lock.IsDisabled(targetSlug) {
			t.Fatalf("IsDisabled(%q) expected true after SetDisabled", targetSlug)
		}
		// Check that other mods are not affected
		for j := 1; j <= modCount; j++ {
			otherSlug := fmt.Sprintf("mod-%02d", j)
			expectedDisabled := (j <= i)
			if lock.IsDisabled(otherSlug) != expectedDisabled {
				t.Fatalf("SetDisabled cross-contamination: %s disabled=%v, expected %v after setting %s",
					otherSlug, lock.IsDisabled(otherSlug), expectedDisabled, targetSlug)
			}
		}
	}

	// 4. Test SetPinned isolated modifications
	for i := 1; i <= modCount; i++ {
		targetSlug := fmt.Sprintf("mod-%02d", i)
		if !lock.SetPinned(targetSlug, true) {
			t.Fatalf("SetPinned(%q, true) failed", targetSlug)
		}
		if !lock.IsPinned(targetSlug) {
			t.Fatalf("IsPinned(%q) expected true after SetPinned", targetSlug)
		}
		for j := 1; j <= modCount; j++ {
			otherSlug := fmt.Sprintf("mod-%02d", j)
			expectedPinned := (j <= i)
			if lock.IsPinned(otherSlug) != expectedPinned {
				t.Fatalf("SetPinned cross-contamination: %s pinned=%v, expected %v after setting %s",
					otherSlug, lock.IsPinned(otherSlug), expectedPinned, targetSlug)
			}
		}
	}

	// 5. Test AddOrUpdateMod mutation
	lock.AddOrUpdateMod(LockfileMod{
		Slug:          "worldedit-cui-3",
		Name:          "WorldEdit CUI 3 Updated",
		ProjectID:     "we-3",
		VersionNumber: "1.2.0",
		FileName:      "worldedit-cui-3-fabric-1.2.0.jar",
	})
	we2 := lock.GetMod("worldedit-cui-2")
	we3 := lock.GetMod("worldedit-cui-3")
	if we2 == nil || we2.Name != "WorldEdit CUI 2" {
		t.Fatalf("worldedit-cui-2 corrupted by updating worldedit-cui-3: %+v", we2)
	}
	if we3 == nil || we3.Name != "WorldEdit CUI 3 Updated" || we3.VersionNumber != "1.2.0" {
		t.Fatalf("worldedit-cui-3 was not updated properly: %+v", we3)
	}

	// 6. Test RemoveMod isolation
	if !lock.RemoveMod("jei-1") {
		t.Fatalf("RemoveMod jei-1 failed")
	}
	if lock.GetMod("jei-1") != nil {
		t.Fatalf("jei-1 still found after removal")
	}
	if lock.GetMod("jei-2") == nil {
		t.Fatalf("jei-2 was erroneously removed when removing jei-1!")
	}
}

// TestChallengerIt2_NumberedSlug_NegativeMatchAndSequentialAdd tests:
// 1. Negative lookups for non-existent numbered slugs (e.g. querying "mod-02" when only "mod-01" is installed) MUST return nil.
// 2. Sequential AddOrUpdateMod building a lockfile of numbered mods (mod-01, mod-02, mod-03) MUST NOT overwrite preceding mods.
func TestChallengerIt2_NumberedSlug_NegativeMatchAndSequentialAdd(t *testing.T) {
	// Case 1: Negative match on single mod lockfile
	lockSingle := &Lockfile{
		Mods: []LockfileMod{
			{
				Slug:     "mod-01",
				Name:     "Mod 01",
				FileName: "mod-01-1.0.jar",
			},
		},
	}

	if m := lockSingle.GetMod("mod-02"); m != nil {
		t.Errorf("COLLISION BUG: GetMod(\"mod-02\") returned mod-01 (%+v) instead of nil when only mod-01 is installed!", m)
	}
	if m := lockSingle.GetMod("mod-03"); m != nil {
		t.Errorf("COLLISION BUG: GetMod(\"mod-03\") returned %+v instead of nil!", m)
	}

	// Case 2: Negative match on worldedit-cui and jei
	lockPairs := &Lockfile{
		Mods: []LockfileMod{
			{Slug: "worldedit-cui-2", Name: "WorldEdit CUI 2", FileName: "worldedit-cui-2.jar"},
			{Slug: "jei-1", Name: "JEI 1", FileName: "jei-1.jar"},
		},
	}
	if m := lockPairs.GetMod("worldedit-cui-3"); m != nil {
		t.Errorf("COLLISION BUG: GetMod(\"worldedit-cui-3\") returned worldedit-cui-2 (%+v) instead of nil!", m)
	}
	if m := lockPairs.GetMod("jei-2"); m != nil {
		t.Errorf("COLLISION BUG: GetMod(\"jei-2\") returned jei-1 (%+v) instead of nil!", m)
	}

	// Case 3: Sequential addition of numbered mods (simulating sequential installs)
	seqLock := &Lockfile{
		Mods: []LockfileMod{},
	}
	const count = 5
	for i := 1; i <= count; i++ {
		slug := fmt.Sprintf("mod-%02d", i)
		seqLock.AddOrUpdateMod(LockfileMod{
			Slug:     slug,
			Name:     fmt.Sprintf("Mod %02d", i),
			FileName: fmt.Sprintf("mod-%02d-1.0.jar", i),
		})
		if len(seqLock.Mods) != i {
			t.Fatalf("OVERWRITE BUG: AddOrUpdateMod(%q) overwrote existing entry! Expected %d mods in lockfile, got %d",
				slug, i, len(seqLock.Mods))
		}
	}
}

// TestChallengerIt2_ExtensionStrippingPermutations tests property-based permutations of
// filenames ending in .jar, .jar.disabled, .disabled (without .jar), .zip, .mrpack, and uppercase variants.
func TestChallengerIt2_ExtensionStrippingPermutations(t *testing.T) {
	testMods := []struct {
		slug       string
		basePrefix string
	}{
		{"sodium", "sodium"},
		{"ferrite-core", "ferrite-core"},
		{"cloth-config", "cloth-config"},
		{"appleskin", "appleskin"},
		{"iris", "iris"},
		{"lithium", "lithium"},
		{"jei", "jei"},
		{"worldedit", "worldedit"},
		{"xaero9minimap", "xaero9minimap"},
	}

	loaders := []string{"", "-fabric", "+fabric", "_fabric", "-forge", "+forge", "-quilt", "-neoforge"}
	mcVersions := []string{"", "-mc1.20.1", "+mc1.21", "-1.21.1", "+1.21", "_1.20.4", "+mc1.21.1"}
	semvers := []string{"", "-0.5.8", "-v1.0.0", "-15.0.127", "-2.5.1", "-0.12.0", "-19.0.0.12"}

	extensions := []string{
		".jar",
		".jar.disabled",
		".disabled",
		".zip",
		".zip.disabled",
		".mrpack",
		".mrpack.disabled",
		".JAR",
		".JAR.DISABLED",
		".Disabled",
		".jar.Disabled",
		".jar.disabled.disabled",
	}

	for _, tm := range testMods {
		for _, l := range loaders {
			for _, mc := range mcVersions {
				for _, sv := range semvers {
					for _, ext := range extensions {
						// Build raw filename
						rawName := fmt.Sprintf("%s%s%s%s%s", tm.basePrefix, l, mc, sv, ext)
						extracted := ExtractModSlug(rawName)

						expectedSlug := tm.slug
						if extracted != expectedSlug {
							t.Errorf("ExtractModSlug(%q) = %q, expected %q (loader: %s, mc: %s, sv: %s, ext: %s)",
								rawName, extracted, expectedSlug, l, mc, sv, ext)
						}

						// Test matchMod against active and disabled LockfileMod
						activeMod := &LockfileMod{
							Slug:     expectedSlug,
							Name:     expectedSlug,
							FileName: tm.basePrefix + "-1.0.jar",
						}
						disabledMod := &LockfileMod{
							Slug:     expectedSlug,
							Name:     expectedSlug,
							FileName: tm.basePrefix + "-1.0.jar.disabled",
							Disabled: true,
						}

						if !matchMod(activeMod, rawName) {
							t.Errorf("matchMod(activeMod, %q) returned false, expected true for slug %s", rawName, expectedSlug)
						}
						if !matchMod(disabledMod, rawName) {
							t.Errorf("matchMod(disabledMod, %q) returned false, expected true for slug %s", rawName, expectedSlug)
						}
					}
				}
			}
		}
	}
}

// TestChallengerIt2_RandomizedPropertyStress executes 2,000 randomized slug extraction and lookup queries
// under concurrent execution to verify thread-safety and correctness invariant.
func TestChallengerIt2_RandomizedPropertyStress(t *testing.T) {
	baseSlugs := []string{
		"sodium", "lithium", "iris", "appleskin", "ferrite-core",
		"cloth-config", "fabric-api", "forge-config-api-port",
	}

	loaders := []string{"", "-fabric", "+fabric", "-forge", "+forge", "-neoforge"}
	mcVersions := []string{"", "-mc1.21", "+1.21.1", "-1.20.4", "+mc1.20.1"}
	semvers := []string{"", "-1.0.0", "-0.5.8", "-2.1.0", "-15.0.1"}
	exts := []string{".jar", ".jar.disabled", ".disabled", ".zip", ".mrpack"}

	// Build lockfile with all base slugs
	lock := &Lockfile{
		Mods: make([]LockfileMod, len(baseSlugs)),
	}
	for i, s := range baseSlugs {
		lock.Mods[i] = LockfileMod{
			Slug:     s,
			Name:     s,
			FileName: fmt.Sprintf("%s-1.0.0.jar", s),
		}
	}

	var wg sync.WaitGroup
	const workers = 10
	const queriesPerWorker = 200

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localRng := rand.New(rand.NewSource(int64(workerID * 1000 + 42)))
			for q := 0; q < queriesPerWorker; q++ {
				slugIdx := localRng.Intn(len(baseSlugs))
				targetSlug := baseSlugs[slugIdx]

				l := loaders[localRng.Intn(len(loaders))]
				mc := mcVersions[localRng.Intn(len(mcVersions))]
				sv := semvers[localRng.Intn(len(semvers))]
				ext := exts[localRng.Intn(len(exts))]

				if strings.HasPrefix(targetSlug, "fabric-api") {
					l = "+fabric"
				}
				if strings.HasPrefix(targetSlug, "forge-config-api") {
					l = ""
				}

				query := fmt.Sprintf("%s%s%s%s%s", targetSlug, l, mc, sv, ext)
				extracted := ExtractModSlug(query)

				if extracted != targetSlug {
					t.Errorf("[Worker %d] ExtractModSlug(%q) = %q, expected %q", workerID, query, extracted, targetSlug)
				}
				m := lock.GetMod(query)
				if m == nil || m.Slug != targetSlug {
					t.Errorf("[Worker %d] GetMod(%q) = %+v, expected %q", workerID, query, m, targetSlug)
				}
			}
		}(w)
	}

	wg.Wait()
}
