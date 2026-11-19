package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type conversion struct {
	engine    string
	runtime   string
	input     string
	outputDir string
	gameID    string
	level     int
	art       artwork
	report    func(int, string)
}

func (c conversion) engineCommand(ctx context.Context, temp string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.engine, args...)
	cmd.Dir = filepath.Dir(c.engine)
	cmd.Env = append(os.Environ(), "TEMP="+temp, "TMP="+temp, "TMPDIR="+temp,
		"DOTNET_ROOT_X64="+c.runtime)
	return cmd
}

func gameOutputFolder(parent, id string) (string, error) {
	id = normalizeSerial(id)
	if id == "" {
		return "", errors.New("enter a PS1 game ID such as SLUS-00627")
	}
	return filepath.Join(parent, id), nil
}

func (c conversion) run(ctx context.Context, configure func(*exec.Cmd), publish func(string, string) error) (string, error) {
	outputFolder := c.outputDir
	if c.gameID != "" {
		var err error
		outputFolder, err = gameOutputFolder(c.outputDir, c.gameID)
		if err != nil {
			return "", err
		}
	}
	stage, err := os.MkdirTemp(c.outputDir, ".ps1-eboot-")
	if err != nil {
		return "", fmt.Errorf("create working folder in the output location: %w", err)
	}
	defer os.RemoveAll(stage)

	args := []string{"-i", c.input, "-o", stage, "-f", "EBOOT", "-l", strconv.Itoa(c.level), "-v", "2", "-x"}
	if c.art.icon != nil || c.art.background != nil || len(c.art.motion) > 0 {
		if err := c.art.writeResources(stage); err != nil {
			return "", fmt.Errorf("prepare artwork: %w", err)
		}
		args = append(args, "--import", "--resource-root", stage, "--resource-format", "%RESOURCE%.%EXT%")
	}
	cmd := c.engineCommand(ctx, stage, args...)
	configure(cmd)

	output := &conversionOutput{report: c.report}
	cmd.Stdout, cmd.Stderr = output, output
	err = cmd.Run()
	output.finish()
	details := strings.TrimSpace(strings.ReplaceAll(output.log.String(), "\b", ""))
	if ctx.Err() != nil {
		return details, ctx.Err()
	}
	if err != nil {
		return details, fmt.Errorf("PSXPackager could not convert the disc: %w", err)
	}

	if c.report != nil {
		c.report(99, "Checking and saving EBOOT.PBP")
	}
	source := filepath.Join(stage, "EBOOT.pbp")
	if err := validatePBP(source); err != nil {
		return details, fmt.Errorf("conversion produced no valid EBOOT.PBP: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return details, err
	}
	createdFolder := false
	if outputFolder != c.outputDir {
		if err := os.Mkdir(outputFolder, 0755); err == nil {
			createdFolder = true
		} else if !os.IsExist(err) {
			return details, fmt.Errorf("create game ID folder: %w", err)
		} else if info, err := os.Lstat(outputFolder); err != nil || !info.IsDir() {
			return details, errors.New("the game ID path is already used by another item")
		}
	}
	if err := publish(source, filepath.Join(outputFolder, "EBOOT.PBP")); err != nil {
		if createdFolder {
			_ = os.Remove(outputFolder)
		}
		return details, fmt.Errorf("save EBOOT.PBP: %w", err)
	}
	return details, nil
}

type conversionOutput struct {
	pending     []byte
	log         bytes.Buffer
	report      func(int, string)
	lastPercent int
}

func (o *conversionOutput) Write(p []byte) (int, error) {
	o.pending = append(o.pending, p...)
	for {
		end := bytes.IndexByte(o.pending, '\n')
		if end < 0 {
			break
		}
		o.accept(string(o.pending[:end]))
		o.pending = o.pending[end+1:]
	}
	return len(p), nil
}

func (o *conversionOutput) finish() {
	if len(o.pending) != 0 {
		o.accept(string(o.pending))
		o.pending = nil
	}
}

func (o *conversionOutput) accept(line string) {
	line = strings.TrimSuffix(line, "\r")
	if value, ok := strings.CutPrefix(line, "PSX_PROGRESS "); ok {
		percent, err := strconv.Atoi(strings.TrimSuffix(value, "%"))
		if err == nil && o.report != nil {
			percent = min(percent, 99)
			if percent > o.lastPercent {
				o.lastPercent = percent
				o.report(percent, "Writing disc")
			}
		}
		return
	}
	if strings.HasPrefix(line, "Writing Disc ") && o.report != nil {
		o.report(o.lastPercent, "Writing disc")
	}
	o.log.WriteString(line)
	o.log.WriteByte('\n')
}

func validatePBP(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}

	header := make([]byte, 40)
	if _, err := io.ReadFull(file, header); err != nil {
		return err
	}
	if string(header[:4]) != "\x00PBP" {
		return errors.New("invalid PBP signature")
	}

	previous := int64(len(header))
	for i := 8; i < len(header); i += 4 {
		offset := int64(binary.LittleEndian.Uint32(header[i:]))
		if offset < previous || offset >= info.Size() {
			return errors.New("invalid PBP section offset")
		}
		previous = offset
	}
	magic := make([]byte, len("PSISOIMG0000"))
	if _, err := file.ReadAt(magic, previous); err != nil {
		return err
	}
	if string(magic) != "PSISOIMG0000" {
		return errors.New("missing single-disc PS1 image")
	}
	return nil
}
