package worker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// transcode decodes inputPath once and encodes every rendition in a single
// ffmpeg invocation (multiple outputs in one process, each with its own
// scale/bitrate/HLS options) -- far cheaper than re-decoding the source once
// per rendition.
func transcode(ctx context.Context, ffmpegPath, videoCodec, preset, inputPath, outputDir string, renditions []rendition) error {
	if len(renditions) == 0 {
		return fmt.Errorf("no renditions to encode")
	}

	args := []string{"-hide_banner", "-y", "-i", inputPath}

	for _, r := range renditions {
		args = append(args,
			// force_original_aspect_ratio + pad keeps non-16:9 sources
			// (e.g. a phone recording) letterboxed instead of distorted.
			"-vf", fmt.Sprintf(
				"scale=w=%d:h=%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
				r.Width, r.Height, r.Width, r.Height,
			),
			"-c:v", videoCodec,
			"-preset", preset,
			"-profile:v", "main",
			"-crf", "20",
			"-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
			"-b:v", fmt.Sprintf("%dk", r.VideoBitrateKbps),
			"-maxrate", fmt.Sprintf("%dk", r.MaxrateKbps),
			"-bufsize", fmt.Sprintf("%dk", r.BufsizeKbps),
			"-c:a", "aac", "-ar", "48000", "-b:a", fmt.Sprintf("%dk", r.AudioBitrateKbps),
			"-hls_time", "4",
			"-hls_playlist_type", "vod",
			"-hls_flags", "independent_segments",
			"-hls_segment_filename", filepath.Join(outputDir, r.Name+"_%04d.ts"),
			filepath.Join(outputDir, r.Name+".m3u8"),
		)
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg exited with error: %w; stderr(tail): %s", err, tail(stderr.String(), 4000))
	}
	return nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
