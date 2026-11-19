package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	bootLine  = regexp.MustCompile(`(?im)^\s*BOOT\s*=\s*cdrom\d*:[^\r\n;]*?([a-z]{4}[-_]?[0-9]{3}\.?[0-9]{2})\s*(?:;|$)`)
	cueBinary = regexp.MustCompile(`(?im)^\s*FILE\s+(?:"([^"]+)"|(\S+))\s+BINARY\s*$`)
)

func discSerial(path string) (string, error) {
	if strings.EqualFold(filepath.Ext(path), ".cue") {
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(io.LimitReader(file, 1<<20))
		file.Close()
		if err != nil {
			return "", err
		}
		for _, match := range cueBinary.FindAllStringSubmatch(string(data), -1) {
			name := match[1]
			if name == "" {
				name = match[2]
			}
			name = strings.ReplaceAll(name, "\\", string(os.PathSeparator))
			if !filepath.IsAbs(name) {
				name = filepath.Join(filepath.Dir(path), name)
			}
			if id, err := imageSerial(name); err == nil {
				return id, nil
			}
		}
		return "", errors.New("no PS1 boot ID in the CUE track files")
	}
	return imageSerial(path)
}

func imageSerial(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	for _, layout := range [][2]int64{{2352, 24}, {2352, 16}, {2048, 0}, {2336, 8}, {2448, 24}, {2448, 16}} {
		read := func(lba uint32, length uint32) ([]byte, error) {
			if length == 0 || length > 1<<20 {
				return nil, errors.New("invalid ISO9660 extent")
			}
			data := make([]byte, length)
			for pos := 0; pos < len(data); pos += 2048 {
				offset := (int64(lba)+int64(pos/2048))*layout[0] + layout[1]
				if _, err := f.ReadAt(data[pos:min(pos+2048, len(data))], offset); err != nil {
					return nil, err
				}
			}
			return data, nil
		}
		for lba := uint32(16); lba < 48; lba++ {
			pvd, err := read(lba, 2048)
			if err != nil || string(pvd[1:6]) != "CD001" || pvd[0] == 255 {
				break
			}
			if pvd[0] != 1 || pvd[156] < 34 {
				continue
			}
			root, err := read(binary.LittleEndian.Uint32(pvd[158:162]), binary.LittleEndian.Uint32(pvd[166:170]))
			if err != nil {
				break
			}
			var executableID string
			for i := 0; i < len(root); {
				n := int(root[i])
				if n == 0 {
					i = (i/2048 + 1) * 2048
					continue
				}
				if n < 34 || i+n > len(root) {
					break
				}
				rec := root[i : i+n]
				i += n
				nameEnd := 33 + int(rec[32])
				if nameEnd > len(rec) || rec[25]&2 != 0 {
					continue
				}
				name, _, _ := strings.Cut(strings.ToUpper(string(rec[33:nameEnd])), ";")
				if name == "SYSTEM.CNF" {
					text, err := read(binary.LittleEndian.Uint32(rec[2:6]), binary.LittleEndian.Uint32(rec[10:14]))
					if err == nil {
						if match := bootLine.FindStringSubmatch(string(text)); match != nil {
							if id := normalizeSerial(match[1]); id != "" {
								return id, nil
							}
						}
					}
				}
				if id := normalizeSerial(name); id != "" {
					executableID = id
				}
			}
			if executableID != "" {
				return executableID, nil
			}
			break
		}
	}
	return "", errors.New("no PS1 boot ID found in this image")
}

func titleForSerial(path, id string) string {
	if id == "" {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	scan := bufio.NewScanner(file)
	for scan.Scan() {
		fields := strings.SplitN(scan.Text(), ";", 4)
		if len(fields) >= 3 && strings.EqualFold(fields[0], id) {
			return fields[2]
		}
	}
	return ""
}
