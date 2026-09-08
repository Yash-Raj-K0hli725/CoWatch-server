package cogine

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/charmbracelet/log"
)

func (c *Cogine) downloadFromR2(ctx context.Context, objectKey, dstPath string) error {
	bucket := os.Getenv("BUCKET_NAME")
	if bucket == "" {
		log.Error("missing bucket name")
		return errors.New("missing bucket name")
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

func (c *Cogine) uploadToR2(ctx context.Context, srcPath, objectKey string) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()
	bucket := os.Getenv("BUCKET_NAME")
	if bucket == "" {
		log.Error("missing bucket name")
		return errors.New("missing bucket name")
	}

	_, err = c.r2Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(objectKey),
		Body:        file,
		ContentType: aws.String("video/mp4"),
	})
	return err
}
