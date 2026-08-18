package storage

import "github.com/aws/aws-sdk-go-v2/service/s3"

// R2MediaService wraps an S3-compatible client (Cloudflare R2 today) for
// both the presigned-upload path (client -> bucket, API never touches the
// bytes) and the plain-client path the worker fleet uses to download
// sources and upload transcoded HLS artifacts.
type R2MediaService struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucketName    string
}

func NewR2MediaService(s3Client *s3.Client, bucketName string) *R2MediaService {
	return &R2MediaService{
		client:        s3Client,
		presignClient: s3.NewPresignClient(s3Client),
		bucketName:    bucketName,
	}
}
