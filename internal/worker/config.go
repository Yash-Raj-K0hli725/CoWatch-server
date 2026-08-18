// Package worker implements the Go worker side of the pipeline: pull a
// TranscodeJob off the queue, download the source from the bucket, probe
// it, transcode it into an adaptive-bitrate HLS ladder with ffmpeg, upload
// the artifacts to the CDN origin bucket, and publish the outcome for the
// Co-Watch API/websocket server to relay to the room.
package worker

import "time"

// Config holds every tunable of the download -> transcode -> upload
// pipeline plus the queue-consumption knobs the Worker loop uses. It is
// shared, read-only, between Worker and Processor.
type Config struct {
	// Concurrency is how many jobs this process transcodes in parallel.
	// ffmpeg is CPU/GPU heavy, so this should track available cores (or
	// GPU encode sessions), not goroutine-cheap defaults.
	Concurrency int

	// VisibilityTimeout is how long a delivery may sit unacknowledged
	// before Reclaim treats it as abandoned (worker crashed / hung) and
	// makes it available again. Set this comfortably above your
	// worst-case single-job processing time (download + transcode every
	// rendition + upload) -- too short and a slow-but-healthy job gets
	// reclaimed and double-processed by another worker while it's still
	// running.
	VisibilityTimeout time.Duration
	// MaxDeliveries is how many times a job may be (re)delivered before
	// it's moved to the dead-letter stream instead of retried again.
	MaxDeliveries int64
	// ReclaimInterval is how often the background reclaim loop scans for
	// abandoned/failed deliveries.
	ReclaimInterval time.Duration

	// TempDir is the scratch directory each job downloads/transcodes into.
	// Every job gets its own subdirectory, removed when the job finishes
	// (success or failure) so disk usage stays bounded.
	TempDir string
	// MaxDownloadBytes caps how large a source object we'll download; 0
	// disables the cap. Guards against a corrupt/lied-about size field
	// filling the worker's disk.
	MaxDownloadBytes int64

	FFmpegPath  string
	FFprobePath string
	// VideoCodec is the ffmpeg -c:v value, e.g. "libx264" (portable,
	// software) or "h264_nvenc" (GPU-accelerated, needs an NVIDIA GPU +
	// drivers in the worker's environment).
	VideoCodec string
	// Preset is the ffmpeg -preset value. Its valid values depend on
	// VideoCodec: libx264/libx265 take "ultrafast".."veryslow"; h264_nvenc
	// takes "p1".."p7". Defaults to "veryfast" for libx264, a reasonable
	// throughput/quality tradeoff for a multi-rendition batch job.
	Preset string

	// OriginBucket is where finished HLS artifacts are uploaded -- the
	// CDN-fronted bucket, which may or may not be the same bucket the raw
	// source uploads land in.
	OriginBucket string
	// PublicCDNBaseURL is prefixed onto the uploaded object key to build
	// the master playlist URL handed back to the room, e.g.
	// "https://cdn.myapp.com".
	PublicCDNBaseURL string
}
