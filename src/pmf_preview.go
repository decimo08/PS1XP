package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"os/exec"
	"strings"
)

const pmfPreviewFPS = 15
const pmfPreviewMaxFrames = 300

func decodePMFPreview(ctx context.Context, ffmpeg string, pmf []byte, configure func(*exec.Cmd)) ([]byte, error) {
	if len(pmf) < 16 || string(pmf[:4]) != "PSMF" {
		return nil, errors.New("invalid PMF header")
	}
	offset := uint64(binary.BigEndian.Uint32(pmf[8:12]))
	length := uint64(binary.BigEndian.Uint32(pmf[12:16]))
	if offset < 16 || length == 0 || offset+length > uint64(len(pmf)) {
		return nil, errors.New("invalid PMF video payload")
	}

	cmd := exec.CommandContext(ctx, ffmpeg,
		"-nostdin", "-hide_banner", "-v", "error", "-f", "mpeg", "-i", "pipe:0",
		"-map", "0:v:0", "-an",
		"-vf", "fps=15,scale=144:80:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=144:80:(ow-iw)/2:(oh-ih)/2,setsar=1",
		"-frames:v", fmt.Sprint(pmfPreviewMaxFrames), "-threads:v", "1", "-pix_fmt", "rgba", "-f", "rawvideo", "pipe:1")
	cmd.Stdin = bytes.NewReader(pmf[offset : offset+length])
	var details bytes.Buffer
	cmd.Stderr = &details
	configure(cmd)
	raw, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("could not decode the PMF: %w: %s", err, strings.TrimSpace(details.String()))
	}
	if len(raw) == 0 || len(raw)%pmfFrameSize() != 0 {
		return nil, errors.New("the PMF did not contain a complete video frame")
	}
	return raw, nil
}

func pmfFrameSize() int {
	return iconBounds.Dx() * iconBounds.Dy() * 4
}

func pmfPreviewFrame(raw []byte, index int) *image.NRGBA {
	size := pmfFrameSize()
	return &image.NRGBA{Pix: raw[index*size : (index+1)*size], Stride: iconBounds.Dx() * 4, Rect: iconBounds}
}
