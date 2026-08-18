package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"StreamRoom/internal/queue"
	"StreamRoom/storage"
)

// Result is what a successfully processed job produces: the public
// playback URL and the list of renditions actually generated.
type Result struct {
	MasterPlaylistURL string
	Qualities         []string
}

// Processor runs the actual download -> probe -> transcode -> upload
// pipeline for a single job. It holds no per-job state, so one Processor
// is safely shared across all of a Worker's concurrent goroutines.
type Processor struct {
	media *storage.R2MediaService
	cfg   Config
}

func NewProcessor(media *storage.R2MediaService, cfg Config) *Processor {
	return &Processor{media: media, cfg: cfg}
}

// Process runs the full pipeline for one job:
//  1. Download the source video
//  2. ffmpeg transcode into an adaptive HLS ladder (1080p/720p/480p/360p,
//     trimmed to whatever fits under the source's native resolution)
//  3. Generate the HLS master playlist
//  4. Upload every artifact to the CDN origin bucket
//
// All scratch files live under a per-job temp directory that's always
// cleaned up, success or failure.
func (p *Processor) Process(ctx context.Context, job queue.TranscodeJob) (*Result, error) {
	workDir, err := os.MkdirTemp(p.cfg.TempDir, "streamroom-job-*")
	if err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	ext := filepath.Ext(job.ObjectKey)
	if ext == "" {
		ext = ".mp4"
	}
	sourcePath := filepath.Join(workDir, "source"+ext)

	if err := downloadSource(ctx, p.media, p.cfg, job, sourcePath); err != nil {
		return nil, fmt.Errorf("download source: %w", err)
	}

	info, err := probeSource(ctx, p.cfg.FFprobePath, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("probe source: %w", err)
	}

	renditions := ladderFor(info.Width, info.Height)
	if len(renditions) == 0 {
		return nil, fmt.Errorf("could not determine a rendition ladder for %dx%d source", info.Width, info.Height)
	}

	outputDir := filepath.Join(workDir, "hls")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	if err := transcode(ctx, p.cfg.FFmpegPath, p.cfg.VideoCodec, p.cfg.Preset, sourcePath, outputDir, renditions); err != nil {
		return nil, fmt.Errorf("transcode: %w", err)
	}

	masterPath, err := writeMasterPlaylist(outputDir, renditions)
	if err != nil {
		return nil, err
	}

	objectPrefix := fmt.Sprintf("videos/%s/%s", job.RoomID, job.JobID)
	if err := uploadArtifacts(ctx, p.media, p.cfg.OriginBucket, objectPrefix, outputDir); err != nil {
		return nil, fmt.Errorf("upload artifacts: %w", err)
	}

	qualities := make([]string, len(renditions))
	for i, r := range renditions {
		qualities[i] = r.Name
	}

	masterURL := fmt.Sprintf("%s/%s/%s",
		strings.TrimRight(p.cfg.PublicCDNBaseURL, "/"),
		objectPrefix,
		filepath.Base(masterPath),
	)

	return &Result{MasterPlaylistURL: masterURL, Qualities: qualities}, nil
}
