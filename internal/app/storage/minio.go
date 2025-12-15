package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOStorage struct {
	client     *minio.Client
	bucketName string
	endpoint   string
}

func NewMinIOStorage() (*MinIOStorage, error) {
	minioEndpoint := getEnv("MINIO_ENDPOINT", "localhost:9000")
	nginxEndpoint := getEnv("MINIO_PUBLIC_ENDPOINT", "localhost:9000")
	accessKeyID := getEnv("MINIO_ACCESS_KEY", "cavi_admin")
	secretAccessKey := getEnv("MINIO_SECRET_KEY", "cavi_password123")
	bucketName := getEnv("MINIO_BUCKET", "cavi-images")

	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %v", err)
	}

	storage := &MinIOStorage{
		client:     minioClient,
		bucketName: bucketName,
		endpoint:   nginxEndpoint,
	}

	err = storage.createBucketIfNotExists()
	if err != nil {
		return nil, fmt.Errorf("failed to create bucket: %v", err)
	}

	return storage, nil
}

func (s *MinIOStorage) createBucketIfNotExists() error {
	ctx := context.Background()

	exists, err := s.client.BucketExists(ctx, s.bucketName)
	if err != nil {
		return err
	}

	if !exists {
		err = s.client.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return err
		}

		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": {"AWS": "*"},
					"Action": ["s3:GetObject"],
					"Resource": ["arn:aws:s3:::%s/*"]
				}
			]
		}`, s.bucketName)

		err = s.client.SetBucketPolicy(ctx, s.bucketName, policy)
		if err != nil {
			log.Printf("Warning: failed to set bucket policy: %v", err)
		}

		log.Printf("Created bucket '%s'", s.bucketName)
	}

	return nil
}

func (s *MinIOStorage) GetImageURL(filename string) string {
	if filename == "" {
		return ""
	}
	return fmt.Sprintf("http://%s/%s/diagrams/%s", s.endpoint, s.bucketName, filename)
}

func (s *MinIOStorage) GetImageURLByID(id int) string {
	return fmt.Sprintf("http://%s/%s/diagrams/%d.jpg", s.endpoint, s.bucketName, id)
}

func (s *MinIOStorage) GetDefaultImageURL() string {
	return "/static/img/default.jpg"
}

func (s *MinIOStorage) UploadGroupImage(ctx context.Context, id int, reader io.Reader, size int64, contentType string) error {
	objectName := fmt.Sprintf("diagrams/%d.jpg", id)
	opts := minio.PutObjectOptions{ContentType: contentType}
	_, err := s.client.PutObject(ctx, s.bucketName, objectName, reader, size, opts)
	return err
}

func (s *MinIOStorage) DeleteGroupImage(ctx context.Context, id int) error {
	objectName := fmt.Sprintf("diagrams/%d.jpg", id)
	return s.client.RemoveObject(ctx, s.bucketName, objectName, minio.RemoveObjectOptions{})
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
