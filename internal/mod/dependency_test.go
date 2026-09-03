package mod

import (
	"testing"

	"cmm/internal/config"
)

func TestDetectActiveDependents_ActiveAndDisabled(t *testing.T) {
	server, client := setupMockModrinthServer(t)
	defer server.Close()

	lock := &config.Lockfile{
		Mods: []config.LockfileMod{
			{
				Slug:          "sodium",
				Name:          "Sodium",
				ProjectID:     "AANobbMI",
				VersionNumber: "0.5.8",
				FileName:      "sodium-fabric-0.5.8.jar",
				Disabled:      false,
			},
			{
				Slug:          "fabric-api",
				Name:          "Fabric API",
				ProjectID:     "P7dR8mSH",
				VersionNumber: "0.100.0",
				FileName:      "fabric-api-0.100.0.jar",
				Disabled:      false,
			},
		},
	}

	resolver := NewDependencyResolver(client)

	// 1. When sodium is active, fabric-api has sodium as an active dependent
	deps, err := resolver.DetectActiveDependents("fabric-api", lock)
	if err != nil {
		t.Fatalf("DetectActiveDependents failed: %v", err)
	}
	if len(deps) != 1 || deps[0] != "sodium" {
		t.Errorf("expected ['sodium'] as active dependent, got %v", deps)
	}

	// Also verify lookup by project ID
	depsPID, err := resolver.DetectActiveDependents("P7dR8mSH", lock)
	if err != nil {
		t.Fatalf("DetectActiveDependents by project ID failed: %v", err)
	}
	if len(depsPID) != 1 || depsPID[0] != "sodium" {
		t.Errorf("expected ['sodium'] as active dependent when queried by project ID, got %v", depsPID)
	}

	// 2. When sodium is disabled, fabric-api has NO active dependents
	lock.Mods[0].Disabled = true
	deps, err = resolver.DetectActiveDependents("fabric-api", lock)
	if err != nil {
		t.Fatalf("DetectActiveDependents failed: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 active dependents when dependent mod is disabled, got %v", deps)
	}

	// 3. Sodium itself has no dependents
	deps, err = resolver.DetectActiveDependents("sodium", lock)
	if err != nil {
		t.Fatalf("DetectActiveDependents failed: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 active dependents for sodium, got %v", deps)
	}

	// 4. Unknown mod returns error
	_, err = resolver.DetectActiveDependents("non-existent", lock)
	if err == nil {
		t.Errorf("expected error for non-existent mod, got nil")
	}

	// 5. Nil lockfile or nil client handles gracefully
	nilDeps, err := resolver.DetectActiveDependents("fabric-api", nil)
	if err != nil || len(nilDeps) != 0 {
		t.Errorf("expected nil deps for nil lockfile, got %v, err: %v", nilDeps, err)
	}

	emptyLock := &config.Lockfile{Mods: []config.LockfileMod{}}
	emptyDeps, err := resolver.DetectActiveDependents("fabric-api", emptyLock)
	if err != nil || len(emptyDeps) != 0 {
		t.Errorf("expected nil deps for empty lockfile, got %v, err: %v", emptyDeps, err)
	}

	nilClientResolver := NewDependencyResolver(nil)
	nilClientDeps, err := nilClientResolver.DetectActiveDependents("sodium", lock)
	if err != nil || len(nilClientDeps) != 0 {
		t.Errorf("expected nil deps for nil client resolver, got %v, err: %v", nilClientDeps, err)
	}
}
