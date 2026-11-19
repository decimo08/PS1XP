package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestImportOfficialResources(t *testing.T) {
	fixture := os.Getenv("PS1XP_OFFICIAL_RESOURCE_ZIP")
	if fixture == "" {
		t.Skip("set PS1XP_OFFICIAL_RESOURCE_ZIP to the official PSXPackager v1.8.0 ZIP")
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := importEngineResources(dir, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if !resourcesReady(dir) {
		t.Fatal("expected imported resources to pass verification")
	}
	if err := os.WriteFile(filepath.Join(dir, "DATA.PSP"), []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if resourcesReady(dir) {
		t.Fatal("damaged boot loader accepted")
	}
	if err := importEngineResources(dir, bytes.NewReader(data)); err != nil || !resourcesReady(dir) {
		t.Fatalf("re-import did not repair resources: %v", err)
	}
}

func TestRejectInvalidResourceArchive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "resources")
	if err := importEngineResources(dir, bytes.NewReader([]byte("not a zip"))); err == nil {
		t.Fatal("invalid ZIP accepted")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("invalid import changed destination: %v", err)
	}
}
