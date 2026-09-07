package service

import (
	"StreamRoom/internal/views"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/charmbracelet/log"
)

type Cogine struct {
	r2Client *s3.Client
}

func (c *Cogine) downloadFromR2(ctx context.Context, objectKey, dstPath string) error {
	bucket := os.Getenv("BUCKET_NAME")
	if bucket == "" {
		log.Errorf("missing bucket name")
	}
	resp, err := c.r2Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (c *Cogine) StartCompression(ctx context.Context, request views.TaskRequest) error {
	tmpDir, err := os.MkdirTemp("", "video-proc-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) // Automatic cleanup

	inputPath := filepath.Join(tmpDir, "input.mp4")
	outputPath := filepath.Join(tmpDir, "output.mp4")

	// 1. Download file from Cloudflare R2
	log.Printf("[ROOM %s] Downloading %s from R2...", request.ID, request.Obzect)
	if err := c.downloadFromR2(ctx, request.Obzect, inputPath); err != nil {
		return fmt.Errorf("R2 download failed: %w", err)
	}

	// 2. Transcode with CPU rendering and real-time progress parsing
	log.Printf("[ROOM %s] Starting FFmpeg compression...", request.ID)
	if err := c.runFFmpegWithProgress(ctx, request.ID, inputPath, outputPath); err != nil {
		return fmt.Errorf("FFmpeg encoding failed: %w", err)
	}

	// 3. Upload encoded video back to R2
	compressedKey := fmt.Sprintf("compressed/%s_%s", job.TargetRes, job.ObjectKey)
	log.Printf("[Worker %d] Uploading compressed video to R2 as %s...", workerID, compressedKey)
	if err := wp.uploadToR2(ctx, outputPath, compressedKey); err != nil {
		return fmt.Errorf("R2 upload failed: %w", err)
	}

	return nil
}
