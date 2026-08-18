package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

// sourceInfo is what we need from the source file before deciding how to
// transcode it.
type sourceInfo struct {
	Width, Height int
	DurationSec   float64
}

// probeSource runs ffprobe against the downloaded source to read its
// resolution (used to pick the rendition ladder -- see ladderFor) and
// duration (useful for logging/metrics/future progress reporting).
func probeSource(ctx context.Context, ffprobePath, path string) (*sourceInfo, error) {
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-show_entries", "format=duration",
		"-of", "json",
		path,
	}
	cmd := exec.CommandContext(ctx, ffprobePath, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var parsed struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}
	if len(parsed.Streams) == 0 {
		return nil, fmt.Errorf("no video stream found in source")
	}

	dur, _ := strconv.ParseFloat(parsed.Format.Duration, 64)
	return &sourceInfo{
		Width:       parsed.Streams[0].Width,
		Height:      parsed.Streams[0].Height,
		DurationSec: dur,
	}, nil
}
