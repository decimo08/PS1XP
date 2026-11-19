package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/bodgit/sevenzip"
)

const ffmpegBuild = "ffmpeg-9.0.2-essentials_build"

const ffmpegArchiveDir = "FFmpeg"

func prepareFFmpeg(ctx context.Context, toolsDir string) (string, error) {
	dir := filepath.Join(toolsDir, ffmpegArchiveDir)
	bin := filepath.Join(dir, ffmpegBuild)
	names := []string{"ffmpeg.exe", "ffprobe.exe"}
	installed := func() bool {
		for _, name := range names {
			info, err := os.Stat(filepath.Join(bin, name))
			if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
				return false
			}
		}
		return true
	}
	exe := filepath.Join(bin, names[0])
	if installed() {
		return exe, nil
	}

	archive, err := sevenzip.OpenReader(filepath.Join(dir, ffmpegBuild+".7z"))
	if err != nil {
		return "", fmt.Errorf("could not open bundled FFmpeg archive: %w", err)
	}
	defer archive.Close()
	stage, err := os.MkdirTemp(dir, ".ffmpeg-")
	if err != nil {
		return "", fmt.Errorf("could not prepare the FFmpeg folder: %w", err)
	}
	defer os.RemoveAll(stage)

	for _, name := range names {
		archiveName := path.Join(ffmpegBuild, "bin", name)
		src, err := archive.Open(archiveName)
		if err != nil {
			return "", fmt.Errorf("could not read %s from the bundle: %w", name, err)
		}
		dst, err := os.Create(filepath.Join(stage, name))
		if err != nil {
			src.Close()
			return "", err
		}
		_, copyErr := io.Copy(dst, contextReader{ctx: ctx, reader: src})
		if err := errors.Join(copyErr, src.Close(), dst.Close()); err != nil {
			return "", fmt.Errorf("could not unpack %s: %w", name, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.Rename(stage, bin); err != nil && !installed() {
		return "", fmt.Errorf("could not finish preparing bundled FFmpeg: %w", err)
	}
	return exe, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
