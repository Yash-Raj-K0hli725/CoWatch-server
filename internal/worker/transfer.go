package worker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"StreamRoom/internal/queue"
	"StreamRoom/storage"
)

// limitedWriter aborts once more than limit bytes have been written --
// defense in depth against a lied-about (or stale) SizeBytes on the job:
// we don't just skip the pre-check, we cap the actual stream too.
type limitedWriter struct {
	w     io.Writer
	limit int64
	n     int64
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	l.n += int64(len(p))
	if l.n > l.limit {
		return 0, fmt.Errorf("download exceeded max allowed size of %d bytes", l.limit)
	}
	return l.w.Write(p)
}

// downloadSource fetches job's source object into dest, enforcing
// cfg.MaxDownloadBytes both up front (against the job's reported size) and
// as a hard cap on the actual byte stream.
func downloadSource(ctx context.Context, media *storage.R2MediaService, cfg Config, job queue.TranscodeJob, dest string) error {
	if cfg.MaxDownloadBytes > 0 && job.SizeBytes > cfg.MaxDownloadBytes {
		return fmt.Errorf("source object is %d bytes, exceeds max allowed %d", job.SizeBytes, cfg.MaxDownloadBytes)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}
	defer f.Close()

	var w io.Writer = f
	if cfg.MaxDownloadBytes > 0 {
		w = &limitedWriter{w: f, limit: cfg.MaxDownloadBytes}
	}

	n, err := media.DownloadTo(ctx, job.ObjectKey, w)
	if err != nil {
		return fmt.Errorf("download %s: %w", job.ObjectKey, err)
	}
	if n == 0 {
		return fmt.Errorf("downloaded 0 bytes for %s", job.ObjectKey)
	}
	return nil
}

// uploadArtifacts uploads every file in dir (HLS segments + variant
// playlists + the master playlist) to bucket under objectPrefix, bounding
// how many uploads run concurrently so one job can't monopolize the
// worker's outbound bandwidth/connections.
func uploadArtifacts(ctx context.Context, media *storage.R2MediaService, bucket, objectPrefix, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read output dir: %w", err)
	}

	const maxConcurrentUploads = 8
	sem := make(chan struct{}, maxConcurrentUploads)
	var wg sync.WaitGroup
	errCh := make(chan error, len(entries))

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			path := filepath.Join(dir, name)
			if err := uploadFile(ctx, media, bucket, objectPrefix+"/"+name, path); err != nil {
				errCh <- fmt.Errorf("%s: %w", name, err)
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			// Surface the first error; the whole job is retried (or
			// eventually dead-lettered) rather than left half-uploaded,
			// since a partially-uploaded HLS set is worse than none --
			// clients would 404 on missing segments mid-playback.
			return err
		}
	}
	return nil
}

func uploadFile(ctx context.Context, media *storage.R2MediaService, bucket, objectKey, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return media.Upload(ctx, bucket, objectKey, contentTypeFor(path), f, info.Size())
}

func contentTypeFor(path string) string {
	switch filepath.Ext(path) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	default:
		return "application/octet-stream"
	}
}
