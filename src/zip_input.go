package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const maxDiscArchiveSize uint64 = 4 << 30

var archiveCueFile = regexp.MustCompile(`(?i)^\s*FILE\s+(?:"([^"]+)"|(\S+))\s+BINARY\s*$`)

func archivePath(name string) (string, error) {
	name = strings.TrimSuffix(strings.ReplaceAll(name, "\\", "/"), "/")
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || part != strings.TrimRight(part, " .") ||
			strings.ContainsAny(part, ":<>\"|?*") || strings.IndexFunc(part, func(r rune) bool { return r < 32 }) >= 0 ||
			!filepath.IsLocal(part) {
			return "", fmt.Errorf("unsupported archive path %q", name)
		}
	}
	return name, nil
}

func extractDiscArchive(ctx context.Context, file, dir string) (string, error) {
	archive, err := zip.OpenReader(file)
	if err != nil {
		return "", fmt.Errorf("open disc ZIP: %w", err)
	}
	defer archive.Close()
	if len(archive.File) > 1024 {
		return "", errors.New("the disc ZIP contains too many entries")
	}
	var cues, images []string
	files := make(map[string]*zip.File)
	var total uint64
	for _, item := range archive.File {
		name, err := archivePath(item.Name)
		if err != nil {
			return "", err
		}
		if item.FileInfo().IsDir() || strings.HasPrefix(name, "__MACOSX/") {
			continue
		}
		if !item.Mode().IsRegular() {
			return "", fmt.Errorf("the ZIP contains an unsupported entry: %s", name)
		}
		ext := strings.ToLower(path.Ext(name))
		if ext != ".cue" && ext != ".bin" && ext != ".iso" && ext != ".img" {
			continue
		}
		key := strings.ToLower(name)
		if files[key] != nil {
			return "", fmt.Errorf("the ZIP has duplicate names for %s", name)
		}
		if item.UncompressedSize64 > maxDiscArchiveSize-total {
			return "", errors.New("disc images in this ZIP exceed 4 GiB")
		}
		total += item.UncompressedSize64
		files[key] = item
		switch ext {
		case ".cue":
			cues = append(cues, name)
		case ".bin", ".iso":
			images = append(images, name)
		}
	}

	var chosen string
	switch {
	case len(cues) == 1:
		chosen = cues[0]
	case len(cues) == 0 && len(images) == 1:
		chosen = images[0]
	default:
		return "", errors.New("use a ZIP with one CUE or a single BIN or ISO")
	}

	output := filepath.Join(dir, "disc"+strings.ToLower(path.Ext(chosen)))
	if len(cues) == 0 {
		return output, extractArchiveEntry(ctx, files[strings.ToLower(chosen)], output)
	}

	cue := files[strings.ToLower(chosen)]
	if cue.UncompressedSize64 > 1<<20 {
		return "", errors.New("the ZIP contains an oversized CUE sheet")
	}
	in, err := cue.Open()
	if err != nil {
		return "", err
	}
	raw, readErr := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, reader: in}, (1<<20)+1))
	if err := errors.Join(readErr, in.Close()); err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimPrefix(string(raw), "\ufeff"), "\n")
	tracks := make(map[string]string)
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.EqualFold(fields[0], "TRACK") || strings.EqualFold(fields[0], "INDEX") {
			lines[i] = "  " + strings.ToUpper(strings.Join(fields, " "))
			continue
		}
		if !strings.EqualFold(fields[0], "FILE") {
			continue
		}
		match := archiveCueFile.FindStringSubmatch(line)
		if match == nil {
			return "", errors.New("ZIP CUE sheets must reference BINARY tracks")
		}
		name, err := archivePath(match[1] + match[2])
		if err != nil {
			return "", err
		}
		key := strings.ToLower(path.Join(path.Dir(chosen), name))
		entry := files[key]
		if entry == nil || strings.EqualFold(path.Ext(name), ".cue") {
			return "", fmt.Errorf("the ZIP is missing the CUE track %s", name)
		}
		staged := tracks[key]
		if staged == "" {
			staged = fmt.Sprintf("track-%02d.bin", len(tracks)+1)
			if err := extractArchiveEntry(ctx, entry, filepath.Join(dir, staged)); err != nil {
				return "", err
			}
			tracks[key] = staged
		}
		lines[i] = fmt.Sprintf("FILE \"%s\" BINARY", staged)
	}
	if len(tracks) == 0 {
		return "", errors.New("no BIN tracks were found in the ZIP’s CUE sheet")
	}
	return output, os.WriteFile(output, []byte(strings.Join(lines, "\n")), 0600)
}

func extractArchiveEntry(ctx context.Context, entry *zip.File, output string) error {
	in, err := entry.Open()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		in.Close()
		return err
	}
	_, copyErr := io.Copy(out, contextReader{ctx: ctx, reader: in})
	return errors.Join(copyErr, in.Close(), out.Close())
}
