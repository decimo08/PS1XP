package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

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
