package main

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscZIP(t *testing.T) {
	archive := testDiscZIP(t, map[string]string{
		"Disc/Disc.cue":            "file \"tracks\\track 01.BIN\" BINARY\r\ntrack 01 MODE2/2352\r\nindex 01 00:00:00\r\n",
		"Disc/Tracks/Track 01.bin": "track data",
		"Disc/readme.txt":          "ignored",
	})
	dir := t.TempDir()
	input, err := extractDiscArchive(context.Background(), archive, dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(input)
	if err != nil || !strings.Contains(string(data), "FILE \"track-01.bin\" BINARY") {
		t.Fatalf("rewritten CUE: %q, %v", data, err)
	}
	track, err := os.ReadFile(filepath.Join(dir, "track-01.bin"))
	if err != nil || string(track) != "track data" {
		t.Fatalf("staged track: %q, %v", track, err)
	}
}

func TestZIPRejectsInvalidInputs(t *testing.T) {
	for _, items := range []map[string]string{
		{"../outside.bin": "outside"},
		{"Disc.cue": "FILE \"../../outside.bin\" BINARY"},
		{"Disc.cue": "FILE \"missing.bin\" BINARY"},
		{"A.iso": "disc", "B.bin": "disc"},
		{"A.cue": "cue", "B.cue": "cue"},
	} {
		dir := t.TempDir()
		archive := testDiscZIP(t, items)
		if _, err := extractDiscArchive(context.Background(), archive, dir); err == nil {
			t.Fatalf("archive accepted: %v", items)
		}
	}
}

func TestDiscZIPSingleISO(t *testing.T) {
	archive := testDiscZIP(t, map[string]string{"folder/Game.iso": "test ISO"})
	out, err := extractDiscArchive(context.Background(), archive, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil || string(data) != "test ISO" {
		t.Fatalf("staged ISO: %q, %v", data, err)
	}
}

func testDiscZIP(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "disc.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, data := range files {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
