package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const rawSector = 2352

type cutscene struct {
	path          string
	start, end    int64
	width, height int
	frames        int
	channel       byte
}

func scanCutscenes(ctx context.Context, path string) ([]cutscene, error) {
	paths := []string{path}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cue":
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(file, 1<<20))
		file.Close()
		if err != nil {
			return nil, err
		}
		paths = nil
		seen := make(map[string]bool)
		for _, match := range cueBinary.FindAllStringSubmatch(string(data), -1) {
			name := match[1]
			if name == "" {
				name = match[2]
			}
			name = strings.ReplaceAll(name, "\\", string(os.PathSeparator))
			if !filepath.IsAbs(name) {
				name = filepath.Join(filepath.Dir(path), name)
			}
			if !seen[name] {
				paths = append(paths, name)
				seen[name] = true
			}
		}
	case ".bin":
	default:
		return nil, errors.New("drop a raw 2352-byte BIN or its CUE to look for cutscenes")
	}
	var scenes []cutscene
	for _, file := range paths {
		found, err := scanSTRTrack(ctx, file)
		if err != nil {
			return nil, err
		}
		scenes = append(scenes, found...)
	}
	sort.SliceStable(scenes, func(i, j int) bool { return scenes[i].frames > scenes[j].frames })
	if len(scenes) == 0 {
		return nil, errors.New("no standard PS1 STR videos found on this disc")
	}
	return scenes, nil
}

func scanSTRTrack(ctx context.Context, path string) ([]cutscene, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size()%rawSector != 0 {
		return nil, nil
	}

	reader := bufio.NewReaderSize(f, 256*rawSector)
	var sector [rawSector]byte
	syncHeader := []byte{0, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 0}
	type stream struct {
		cutscene
		lastFrame uint32
	}
	var streams [32]stream
	var scenes []cutscene
	save := func(s *stream) {
		if s.frames >= 15 {
			scenes = append(scenes, s.cutscene)
		}
		*s = stream{}
	}
	for n := int64(0); n < stat.Size()/rawSector; n++ {
		if n%256 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if _, err = io.ReadFull(reader, sector[:]); err != nil {
			return nil, err
		}
		if !bytes.Equal(sector[:12], syncHeader) || sector[15] != 2 || sector[17] >= 32 || binary.LittleEndian.Uint32(sector[24:28]) != 0x80010160 {
			continue
		}
		chunk, count := binary.LittleEndian.Uint16(sector[28:30]), binary.LittleEndian.Uint16(sector[30:32])
		width, height := int(binary.LittleEndian.Uint16(sector[40:42])), int(binary.LittleEndian.Uint16(sector[42:44]))
		if chunk >= count || count > 32 || width < 16 || width > 640 || height < 16 || height > 480 {
			continue
		}
		frame := binary.LittleEndian.Uint32(sector[32:36])
		s := &streams[sector[17]]
		if s.frames > 0 && (n-s.end > 150 || frame < s.lastFrame || width != s.width || height != s.height) {
			save(s)
		}
		if s.frames == 0 {
			if chunk != 0 {
				continue
			}
			s.cutscene = cutscene{path: path, start: n, width: width, height: height, channel: sector[17]}
		}
		if chunk == 0 {
			s.frames++
		}
		s.lastFrame = frame
		s.end = n + 1
	}
	for i := range streams {
		save(&streams[i])
	}
	return scenes, nil
}

func makeCutscenePMF(ctx context.Context, scene cutscene, toolsDir, ffmpeg, python string, configure func(*exec.Cmd)) ([]byte, error) {
	dir, err := os.MkdirTemp("", "ps1-cutscene-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	source, err := os.Open(scene.path)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	str := filepath.Join(dir, "cutscene.str")
	out, err := os.Create(str)
	if err != nil {
		return nil, err
	}
	// STR streams are normally about 15 fps. Keep an initial sample for the silent menu loop.
	end := min(scene.end, scene.start+1200)
	reader := bufio.NewReader(io.NewSectionReader(source, scene.start*rawSector, (end-scene.start)*rawSector))
	var sector [rawSector]byte
	var copyErr error
	for n := scene.start; n < end; n++ {
		if _, copyErr = io.ReadFull(reader, sector[:]); copyErr != nil {
			break
		}
		if sector[17] == scene.channel && binary.LittleEndian.Uint32(sector[24:28]) == 0x80010160 {
			if _, copyErr = out.Write(sector[:]); copyErr != nil {
				break
			}
		}
	}
	if err := errors.Join(copyErr, out.Close()); err != nil {
		return nil, err
	}
	mp4, pmf := filepath.Join(dir, "clip.mp4"), filepath.Join(dir, "ICON1.PMF")
	run := func(name string, args ...string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(ffmpeg)+string(os.PathListSeparator)+os.Getenv("PATH"))
		configure(cmd)
		if log, err := cmd.CombinedOutput(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(log)))
		}
		return nil
	}
	if err := run(ffmpeg, "-nostdin", "-v", "error", "-f", "psxstr", "-i", str, "-map", "0:v:0", "-an", "-t", "5", "-c:v", "libx264", "-pix_fmt", "yuv420p", mp4); err != nil {
		return nil, fmt.Errorf("decode cutscene: %w", err)
	}
	if err := run(python, filepath.Join(toolsDir, "pspmedia.py"), "convert", mp4, "-o", pmf, "--budget-kb", "440"); err != nil {
		return nil, fmt.Errorf("encode PMF: %w", err)
	}
	motion, err := readMotion(pmf)
	if err == nil && len(motion) > 500*1024 {
		return nil, errors.New("generated PMF is above the 500 KiB menu budget")
	}
	return motion, err
}
