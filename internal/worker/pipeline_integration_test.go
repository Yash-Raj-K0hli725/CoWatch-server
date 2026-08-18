package worker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// requireFFmpeg skips the test if ffmpeg/ffprobe aren't on PATH (or
// overridden via FFMPEG_PATH/FFPROBE_PATH) -- this test exercises the real
// binaries, not a mock, since the whole point is validating the argument
// lists we hand them.
func requireFFmpeg(t *testing.T) (ffmpegPath, ffprobePath string) {
	t.Helper()
	ffmpegPath = envOrDefault("FFMPEG_PATH", "ffmpeg")
	ffprobePath = envOrDefault("FFPROBE_PATH", "ffprobe")
	if _, err := exec.LookPath(ffmpegPath); err != nil {
		t.Skipf("ffmpeg not available (%s): %v", ffmpegPath, err)
	}
	if _, err := exec.LookPath(ffprobePath); err != nil {
		t.Skipf("ffprobe not available (%s): %v", ffprobePath, err)
	}
	return ffmpegPath, ffprobePath
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// generateSyntheticClip writes a short synthetic 1280x720 test clip using
// ffmpeg's lavfi test sources, so this test needs no fixture file checked
// into the repo.
func generateSyntheticClip(t *testing.T, ffmpegPath, dest string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-hide_banner", "-y",
		"-f", "lavfi", "-i", "testsrc=size=1280x720:rate=24:duration=2",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-shortest",
		dest,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate synthetic test clip: %v\n%s", err, out)
	}
}

// TestPipelineProbeTranscodePlaylist runs the real probe -> ladder trim ->
// ffmpeg transcode -> master playlist chain end-to-end against a
// synthetic clip, without touching the network/queue/storage at all.
func TestPipelineProbeTranscodePlaylist(t *testing.T) {
	ffmpegPath, ffprobePath := requireFFmpeg(t)

	workDir := t.TempDir()
	sourcePath := filepath.Join(workDir, "source.mp4")
	generateSyntheticClip(t, ffmpegPath, sourcePath)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	info, err := probeSource(ctx, ffprobePath, sourcePath)
	if err != nil {
		t.Fatalf("probeSource: %v", err)
	}
	if info.Width != 1280 || info.Height != 720 {
		t.Fatalf("probeSource resolution = %dx%d, want 1280x720", info.Width, info.Height)
	}

	renditions := ladderFor(info.Width, info.Height)
	wantRenditions := []string{"720p", "480p", "360p"}
	if len(renditions) != len(wantRenditions) {
		t.Fatalf("ladderFor(720p source) = %d renditions, want %d", len(renditions), len(wantRenditions))
	}

	outputDir := filepath.Join(workDir, "hls")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output dir: %v", err)
	}

	if err := transcode(ctx, ffmpegPath, "libx264", "ultrafast", sourcePath, outputDir, renditions); err != nil {
		t.Fatalf("transcode: %v", err)
	}

	for _, r := range renditions {
		playlist := filepath.Join(outputDir, r.Name+".m3u8")
		if fi, err := os.Stat(playlist); err != nil || fi.Size() == 0 {
			t.Errorf("expected non-empty variant playlist %s: err=%v", playlist, err)
		}

		matches, _ := filepath.Glob(filepath.Join(outputDir, r.Name+"_*.ts"))
		if len(matches) == 0 {
			t.Errorf("expected at least one HLS segment for rendition %s, found none", r.Name)
		}
	}

	masterPath, err := writeMasterPlaylist(outputDir, renditions)
	if err != nil {
		t.Fatalf("writeMasterPlaylist: %v", err)
	}
	masterBytes, err := os.ReadFile(masterPath)
	if err != nil {
		t.Fatalf("read master playlist: %v", err)
	}
	master := string(masterBytes)
	for _, r := range renditions {
		if !strings.Contains(master, r.Name+".m3u8") {
			t.Errorf("master playlist missing reference to %s.m3u8:\n%s", r.Name, master)
		}
	}
}
