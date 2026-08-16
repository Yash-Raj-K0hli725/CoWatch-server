package storage

import "github.com/aws/aws-sdk-go-v2/service/s3"

type R2MediaService struct {
	presignClient *s3.PresignClient
	bucketName    string
}

func NewR2MediaService(s3Client *s3.Client, bucketName string) *R2MediaService {
	return &R2MediaService{
		presignClient: s3.NewPresignClient(s3Client),
		bucketName:    bucketName,
	}
}
