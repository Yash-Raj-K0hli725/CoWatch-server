package storage

import (
	"context"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/charmbracelet/log"
)

func InitStorage() *s3.Client {
	accessKey := os.Getenv("STORAGE_ACCESS_KEY_ID")
	secretKey := os.Getenv("STORAGE_SECRET_KEY")
	host := os.Getenv("STORAGE_S3_API")
	cleanHost := strings.TrimPrefix(host, "https://")
	cleanHost = strings.TrimSuffix(cleanHost, "/")
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://" + cleanHost)
		o.UsePathStyle = true
	})
}
