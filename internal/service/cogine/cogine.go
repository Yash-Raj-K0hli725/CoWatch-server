package cogine

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Cogine struct {
	r2Client *s3.Client
	FFProbe  string
	FFmpeg   string
}

func NewCogine(client *s3.Client) *Cogine {
	return &Cogine{
		r2Client: client,
		FFProbe:  os.Getenv("FFPROBE_PATH"),
		FFmpeg:   os.Getenv("FFMPEG_PATH"),
	}
}

func (c *Cogine) StartCompression(ctx context.Context, OKey string) error {
	tmpDir, err := os.MkdirTemp("", "remote_*")
	if err != nil {
		return fmt.Errorf("failed to create temp file:: %w", err)
	}
	//defer os.RemoveAll(tmpDir)
	in := filepath.Join(tmpDir, "input.mp4")
	log.Printf("starting to download remote upload :: %s", OKey)
	if err = c.downloadFromR2(ctx, OKey, in); err != nil {
		return fmt.Errorf("R2 failed to download %s :: %w", OKey, err)
	}
	c.transcodeToHLS(in, tmpDir)
	log.Printf("ready to upload the video...")
	//c.uploadToR2(ctx, out,)
	return nil
}

// transcodeToHLS uses CPU (libx264) to scale video into 720p and 480p, packetizing them into 2-second HLS chunks with active progress tracking
func (c *Cogine) transcodeToHLS(inputPath, outputDir string) {
	// Delete the raw input file after transcoding finishes
	defer os.Remove(inputPath)

	// Pre-query duration for calculating percentage progress
	duration, err := c.getVideoDurationSeconds(inputPath)
	if err != nil {
		fmt.Printf("🟡 Warning: Could not extract duration for %s: %v\n", inputPath, err)
		duration = 0
	}

	ffmpegExe := c.FFmpeg
	if ffmpegExe == "" {
		ffmpegExe = "ffmpeg"
	}

	args := []string{
		"-i", inputPath,
		"-hide_banner", "-y",

		// Filtergraph: split into 3 streams
		"-filter_complex", "[0:v]split=3[v_orig][v1][v2]; [v1]scale=w=1280:h=720[v720]; [v2]scale=w=854:h=480[v480]",

		// ----------------------------------------------------
		// Original Quality Output (e.g., 1080p / Source Res)
		// ----------------------------------------------------
		"-map", "[v_orig]",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-profile:v", "high",
		"-preset", "medium",
		"-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-b:v", "5000k", "-maxrate", "5350k", "-bufsize", "7500k",
		"-c:a", "aac", "-ar", "48000", "-b:a", "192k",
		"-hls_time", "2",
		"-hls_playlist_type", "vod",
		"-hls_flags", "single_file",
		filepath.Join(outputDir, "original.m3u8"),

		// ----------------------------------------------------
		// 720p Output
		// ----------------------------------------------------
		"-map", "[v720]",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-profile:v", "main",
		"-preset", "medium",
		"-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-b:v", "2500k", "-maxrate", "2675k", "-bufsize", "3750k",
		"-c:a", "aac", "-ar", "48000", "-b:a", "128k",
		"-hls_time", "2",
		"-hls_playlist_type", "vod",
		"-hls_flags", "single_file",
		filepath.Join(outputDir, "720p.m3u8"),

		// ----------------------------------------------------
		// 480p Output
		// ----------------------------------------------------
		"-map", "[v480]",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-profile:v", "main",
		"-preset", "medium",
		"-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-b:v", "1000k", "-maxrate", "1070k", "-bufsize", "1500k",
		"-c:a", "aac", "-ar", "48000", "-b:a", "96k",
		"-hls_time", "2",
		"-hls_playlist_type", "vod",
		"-hls_flags", "single_file",
		filepath.Join(outputDir, "480p.m3u8"),

		"-progress", "pipe:1",
		"-nostats",
	}

	cmd := exec.Command(ffmpegExe, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("🔴 Failed to attach stdout pipe for %s: %v\n", inputPath, err)
		return
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		fmt.Printf("🔴 CPU Transcoding error starting %s: %v\n", inputPath, err)
		return
	}

	// Active progress parser
	var done = make(chan struct{})
	go func() {
		c.parseFFmpegProgress(stdout, duration, outputDir)
		close(done)
	}()

	if err := cmd.Wait(); err != nil {
		fmt.Printf("🔴 CPU Transcoding error processing %s: %v\n", inputPath, err)
		fmt.Printf("📋 FFmpeg Stderr Output Log:\n%s\n", stderrBuf.String())
		return
	}
	<-done

	// Create master HLS playlist
	width, height, err := c.getVideoDimensions(inputPath)
	if err != nil {
		fmt.Printf("🔴 CPU Transcoding error while getting dimensions %s: %v\n", inputPath, err)
		return
	}
	c.createMasterPlaylist(outputDir, width, height)
	fmt.Printf("✅ CPU Transcoding successfully completed for directory: %s\n", outputDir)
}

// getVideoDimensions extracts width and height using ffprobe
func (c *Cogine) getVideoDimensions(inputPath string) (int, int, error) {
	ffprobeExe := c.FFProbe
	if ffprobeExe == "" {
		ffprobeExe = "ffprobe"
	}
	
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=s=x:p=0",
		inputPath,
	}

	cmd := exec.Command(ffprobeExe, args...)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return 0, 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var width, height int
	_, err := fmt.Sscanf(strings.TrimSpace(out.String()), "%dx%d", &width, &height)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse dimensions: %w", err)
	}

	return width, height, nil
}

// createMasterPlaylist outputs master.m3u8 linking variant playlists
func (c *Cogine) createMasterPlaylist(outputDir string, origWidth, origHeight int) {
	masterContent := fmt.Sprintf(`#EXTM3U
#EXT-X-VERSION:4
#EXT-X-STREAM-INF:BANDWIDTH=5500000,RESOLUTION=%dx%d,NAME="Original"
original.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=2800000,RESOLUTION=1280x720,NAME="720p"
720p.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=1100000,RESOLUTION=854x480,NAME="480p"
480p.m3u8
`, origWidth, origHeight)

	masterPath := filepath.Join(outputDir, "master.m3u8")
	_ = os.WriteFile(masterPath, []byte(masterContent), 0644)
}
