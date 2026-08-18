// Command worker is the Go worker fleet: it pulls TranscodeJobs off the
// shared Redis Stream, transcodes them into adaptive HLS, uploads the
// result to the CDN origin bucket, and publishes the outcome for the
// Co-Watch API/websocket server to relay to the room. Run as many replicas
// of this binary as you want throughput -- they all share one consumer
// group, so Redis hands each pending job to exactly one of them.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"StreamRoom/internal/events"
	"StreamRoom/internal/queue"
	"StreamRoom/internal/rdb"
	"StreamRoom/internal/worker"
	"StreamRoom/storage"
	"StreamRoom/util"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	client := rdb.NewClient()
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("worker: unable to reach redis at %s: %v", os.Getenv("REDIS_ADDR"), err)
	}

	streamName := util.EnvOr("QUEUE_STREAM_NAME", "streamroom:transcode-jobs")
	groupName := util.EnvOr("QUEUE_CONSUMER_GROUP", "transcode-workers")
	consumerName := util.EnvOr("WORKER_ID", util.HostnamePID("worker"))

	q := queue.NewRedisStream(client, streamName, groupName, consumerName)
	if err := q.EnsureGroup(ctx); err != nil {
		log.Fatalf("worker: failed to init queue: %v", err)
	}

	sourceBucket := os.Getenv("BUCKET_NAME")
	originBucket := util.EnvOr("ORIGIN_BUCKET_NAME", sourceBucket)

	r2Client := storage.InitStorage()
	mediaService := storage.NewR2MediaService(r2Client, sourceBucket)
	// mediaService.Upload() takes its target bucket explicitly per call,
	// so one client instance covers both the source bucket (downloads)
	// and the origin bucket (uploads), whether or not they're the same.

	cfg := worker.Config{
		Concurrency:       util.EnvInt("WORKER_CONCURRENCY", 2),
		VisibilityTimeout: util.EnvDuration("WORKER_VISIBILITY_TIMEOUT", 30*time.Minute),
		MaxDeliveries:     int64(util.EnvInt("WORKER_MAX_DELIVERIES", 3)),
		ReclaimInterval:   util.EnvDuration("WORKER_RECLAIM_INTERVAL", time.Minute),
		TempDir:           util.EnvOr("TRANSCODE_TEMP_DIR", os.TempDir()),
		MaxDownloadBytes:  int64(util.EnvInt("MAX_DOWNLOAD_BYTES", 2*1024*1024*1024)), // 2GiB default
		FFmpegPath:        util.EnvOr("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:       util.EnvOr("FFPROBE_PATH", "ffprobe"),
		VideoCodec:        util.EnvOr("FFMPEG_VIDEO_CODEC", "libx264"),
		Preset:            util.EnvOr("FFMPEG_PRESET", "veryfast"),
		OriginBucket:      originBucket,
		PublicCDNBaseURL:  os.Getenv("PUBLIC_CDN_BASE_URL"),
	}

	processor := worker.NewProcessor(mediaService, cfg)
	publisher := events.NewPublisher(client)
	w := worker.New(q, processor, publisher, cfg)

	log.Printf("worker: starting (consumer=%s stream=%s group=%s concurrency=%d codec=%s origin_bucket=%s)",
		consumerName, streamName, groupName, cfg.Concurrency, cfg.VideoCodec, cfg.OriginBucket)

	w.Run(ctx)
	log.Println("worker: shut down")
}
