package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	imagedraw "golang.org/x/image/draw"
)

const maxArtworkBytes = 16 << 20

var (
	iconBounds       = image.Rect(0, 0, 144, 80)
	backgroundBounds = image.Rect(0, 0, 480, 272)
	serialPattern    = regexp.MustCompile(`^([A-Z]{4})[-_]?([0-9]{3})\.?([0-9]{2})$`)
)

type artwork struct {
	icon, background image.Image
	motion           []byte
}

func readArtwork(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxArtworkBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxArtworkBytes {
		return nil, errors.New("choose an image under 16 MB")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, errors.New("choose a valid PNG or JPEG image")
	}
	if cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 24_000_000 {
		return nil, errors.New("choose an image under 24 megapixels")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

func readLocalArtwork(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readArtwork(f)
}

func fitArtwork(src image.Image, bounds image.Rectangle, fill color.Color) *image.NRGBA {
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, image.NewUniform(fill), image.Point{}, draw.Src)
	if src == nil {
		return dst
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	scale := min(float64(bounds.Dx())/float64(w), float64(bounds.Dy())/float64(h))
	w, h = max(1, int(float64(w)*scale+0.5)), max(1, int(float64(h)*scale+0.5))
	x, y := (bounds.Dx()-w)/2, (bounds.Dy()-h)/2
	imagedraw.CatmullRom.Scale(dst, image.Rect(x, y, x+w, y+h), src, src.Bounds(), draw.Over, nil)
	return dst
}

func (a artwork) writeResources(dir string) error {
	if len(a.motion) > 0 {
		if err := os.WriteFile(filepath.Join(dir, "ICON1.pmf"), a.motion, 0600); err != nil {
			return err
		}
	}
	for _, resource := range []struct {
		name string
		img  image.Image
	}{{"ICON0.png", a.icon}, {"PIC1.png", a.background}} {
		if resource.img == nil {
			continue
		}
		f, err := os.Create(filepath.Join(dir, resource.name))
		if err != nil {
			return err
		}
		writeErr := png.Encode(f, resource.img)
		closeErr := f.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return err
		}
	}
	return nil
}

func normalizeSerial(input string) string {
	parts := serialPattern.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(input)))
	if parts == nil {
		return ""
	}
	return parts[1] + "-" + parts[2] + parts[3]
}

func fetchCover(ctx context.Context, input string) (image.Image, error) {
	serial := normalizeSerial(input)
	if serial == "" {
		return nil, errors.New("enter a PS1 game ID such as SLUS-00067")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://raw.githubusercontent.com/xlenore/psx-covers/main/covers/default/"+serial+".jpg", nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no online cover found for %s", serial)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover service returned HTTP %d", resp.StatusCode)
	}
	return readArtwork(resp.Body)
}

func (c conversion) detectSerial(ctx context.Context, configure func(*exec.Cmd)) (string, error) {
	if id, err := discSerial(c.input); err == nil {
		return id, nil
	}
	dir, err := os.MkdirTemp("", "ps1-artwork-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	format := filepath.Join(dir, "%GAMEID%", "%RESOURCE%.%EXT%")
	cmd := c.engineCommand(ctx, dir, "-i", c.input, "--generate", "--resource-format", format, "-v", "1")
	configure(cmd)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if serial := normalizeSerial(entry.Name()); entry.IsDir() && serial != "" && serial != "SLUS-00000" {
			return serial, nil
		}
	}
	return "", errors.New("could not detect a game ID; enter it manually")
}

func readMotion(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 2<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) < 2048 || string(data[:4]) != "PSMF" || len(data) > 2<<20 {
		return nil, errors.New("choose a PSP-compatible PMF animation no larger than 2 MB")
	}
	return data, nil
}

func saveDefaultArtwork(dir, name string, img image.Image) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".artwork-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	encodeErr := png.Encode(file, img)
	if err := errors.Join(encodeErr, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(dir, name))
}
