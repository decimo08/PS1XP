package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const upstreamResourceZip = "https://github.com/RupertAvery/PSXPackager/releases/download/v1.8.0/PsxPackagerGUI.zip"
const maxResourceZip = 64 << 20

var resourceHashes = map[string]string{
	"BASE.PBP":    "de0d743be8e721b4167679de5655c0df99667844e0da31bcd96397b09b192614",
	"DATA.PSP":    "9a995e281433c6671e8fc9efc16a130f865e5db03e2d5e6cfbba29ab34ec2fb8",
	"gameInfo.db": "a6904b801b4a7fdfde9514c685502d3943619dfd1c4af7e36b958ada81ab9975",
}

func resourcesReady(dir string) bool {
	for name, hash := range resourceHashes {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !matchesResource(data, hash) {
			return false
		}
	}
	return true
}

func matchesResource(data []byte, expected string) bool {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == expected
}

func importEngineResources(dir string, source io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(source, maxResourceZip+1))
	if err != nil {
		return err
	}
	if len(data) > maxResourceZip {
		return errors.New("the selected PSXPackager ZIP is too large")
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open the official ZIP: %w", err)
	}

	out := make(map[string][]byte, len(resourceHashes))
	for name, hash := range resourceHashes {
		for _, entry := range archive.File {
			entryName := strings.ToLower(strings.ReplaceAll(entry.Name, "\\", "/"))
			want := "resources/" + strings.ToLower(name)
			if entryName != want && !strings.HasSuffix(entryName, "/"+want) {
				continue
			}
			rc, err := entry.Open()
			if err != nil {
				return err
			}
			blob, readErr := io.ReadAll(io.LimitReader(rc, 4<<20))
			if err := errors.Join(readErr, rc.Close()); err != nil {
				return err
			}
			if !matchesResource(blob, hash) {
				return fmt.Errorf("%s checksum did not match PSXPackager v1.8.0", name)
			}
			out[name] = blob
			break
		}
		if out[name] == nil {
			return fmt.Errorf("the ZIP is missing %s. Choose the official PSXPackager v1.8.0 ZIP", name)
		}
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for name, blob := range out {
		tmp, err := os.CreateTemp(dir, ".import-*")
		if err != nil {
			return err
		}
		_, writeErr := tmp.Write(blob)
		if err := errors.Join(writeErr, tmp.Close()); err != nil {
			_ = os.Remove(tmp.Name())
			return err
		}
		if err := os.Rename(tmp.Name(), filepath.Join(dir, name)); err != nil {
			_ = os.Remove(tmp.Name())
			return err
		}
	}
	return nil
}
