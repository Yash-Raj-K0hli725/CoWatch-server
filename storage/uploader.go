package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	URL_TTL = 15 * time.Minute
)

func (s *R2MediaService) GenerateUploadURL(ctx context.Context, contentType string, objectKey string) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	}

	presignedReq, err := s.presignClient.PresignPutObject(ctx, input, s3.WithPresignExpires(URL_TTL))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned upload url: %w", err)
	}

	return presignedReq.URL, nil
}
