package storage

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type R2Storage struct {
	client     *s3.S3
	bucketName string
}

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	AccessKeySecret string
	BucketName      string
}

func NewR2Storage(config R2Config) (*R2Storage, error) {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", config.AccountID)

	httpClient := &http.Client{Timeout: 60 * time.Second}

	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String("auto"),
		Endpoint: aws.String(endpoint),
		Credentials: credentials.NewStaticCredentials(
			config.AccessKeyID,
			config.AccessKeySecret,
			"",
		),
		S3ForcePathStyle: aws.Bool(true),
		HTTPClient:       httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &R2Storage{
		client:     s3.New(sess),
		bucketName: config.BucketName,
	}, nil
}

func (r *R2Storage) UploadFile(key string, data []byte, contentType string) error {
	_, err := r.client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(r.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to R2: %w", err)
	}
	return nil
}

func (r *R2Storage) DownloadFile(key string) ([]byte, error) {
	result, err := r.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download file from R2: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}
	return data, nil
}

func (r *R2Storage) DeleteFile(key string) error {
	_, err := r.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file from R2: %w", err)
	}
	return nil
}

func (r *R2Storage) FileExists(key string) (bool, error) {
	_, err := r.client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}
	return true, nil
}

func (r *R2Storage) GetFileSize(key string) (int64, error) {
	result, err := r.client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	return *result.ContentLength, nil
}
