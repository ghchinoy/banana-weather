package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
	t.Setenv("GENMEDIA_BUCKET", "test-bucket")
	t.Setenv("GOOGLE_MAPS_API_KEY", "test-key")
}

func TestGenerate_ConflictingFlags(t *testing.T) {
	setTestEnv(t)

	rootCmd.SetArgs([]string{"generate", "--csv", "presets.csv", "--id", "london"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("rootCmd.Execute(generate --csv --id) = nil, want conflicting-flags error")
	}
	if !strings.Contains(err.Error(), "cannot specify both --csv and single-location flags") {
		t.Errorf("rootCmd.Execute(generate --csv --id) error = %q, want message containing 'cannot specify both --csv and single-location flags'", err.Error())
	}
}

func TestGenerate_SingleDryRun(t *testing.T) {
	setTestEnv(t)

	rootCmd.SetArgs([]string{"generate", "--csv", "", "--id", "tokyo", "--name", "Tokyo, Japan", "--city", "Tokyo", "--dry-run"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute(generate single --dry-run) = %v, want nil", err)
	}
}

func TestGenerate_BatchDryRun(t *testing.T) {
	setTestEnv(t)

	tmpCSV := filepath.Join(t.TempDir(), "presets.csv")
	csvContent := "id,name,city,category,context\ntokyo,Tokyo,Tokyo,Cities,\narrakis,Arrakis,Arrakeen,Sci-Fi,Desert planet\n"
	if err := os.WriteFile(tmpCSV, []byte(csvContent), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) = %v", tmpCSV, err)
	}

	rootCmd.SetArgs([]string{"generate", "--csv", tmpCSV, "--id", "", "--name", "", "--city", "", "--dry-run"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute(generate batch --dry-run) = %v, want nil", err)
	}
}

func TestAdminList_InvalidType(t *testing.T) {
	setTestEnv(t)

	rootCmd.SetArgs([]string{"admin", "list", "--type", "invalid"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("rootCmd.Execute(admin list --type invalid) = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "invalid --type") {
		t.Errorf("rootCmd.Execute(admin list --type invalid) error = %q, want 'invalid --type'", err.Error())
	}
}
