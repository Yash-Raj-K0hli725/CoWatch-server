package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ObjectInfo is a minimal, storage-agnostic summary of an object's
// metadata, used to verify an upload actually landed before trusting it
// enough to enqueue a transcode job.
type ObjectInfo struct {
	SizeBytes   int64
	ETag        string
	ContentType string
}

// HeadObject is the server-side trust boundary for client-confirmed
// uploads: never enqueue a transcode job purely because a client *claims*
// its presigned PUT succeeded -- verify the object actually exists first.
func (s *R2MediaService) HeadObject(ctx context.Context, objectKey string) (*ObjectInfo, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("head object %q in bucket %q: %w", objectKey, s.bucketName, err)
	}

	info := &ObjectInfo{}
	if out.ContentLength != nil {
		info.SizeBytes = *out.ContentLength
	}
	if out.ETag != nil {
		info.ETag = strings.Trim(*out.ETag, `"`)
	}
	if out.ContentType != nil {
		info.ContentType = *out.ContentType
	}
	return info, nil
}

// DownloadTo streams the named object's body from this service's source
// bucket into dst, returning the number of bytes copied.
func (s *R2MediaService) DownloadTo(ctx context.Context, objectKey string, dst io.Writer) (int64, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return 0, fmt.Errorf("get object %q in bucket %q: %w", objectKey, s.bucketName, err)
	}
	defer out.Body.Close()
	return io.Copy(dst, out.Body)
}

// Upload puts a single object (an HLS segment, variant playlist, or the
// master playlist) into bucket at objectKey. bucket is passed explicitly
// rather than reusing s.bucketName because transcoded artifacts commonly
// land in a separate CDN origin bucket from the raw source uploads.
func (s *R2MediaService) Upload(ctx context.Context, bucket, objectKey, contentType string, body io.Reader, sizeBytes int64) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(objectKey),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	if sizeBytes > 0 {
		input.ContentLength = aws.Int64(sizeBytes)
	}
	if _, err := s.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("put object %q in bucket %q: %w", objectKey, bucket, err)
	}
	return nil
}
