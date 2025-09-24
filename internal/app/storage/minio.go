package storage

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOStorage struct {
	client     *minio.Client
	bucketName string
	endpoint   string
}

func NewMinIOStorage() (*MinIOStorage, error) {
	endpoint := "localhost:8080"
	accessKeyID := "admin"
	secretAccessKey := "password123"
	bucketName := "images"

	// Инициализация MinIO клиента
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %v", err)
	}

	storage := &MinIOStorage{
		client:     minioClient,
		bucketName: bucketName,
		endpoint:   endpoint,
	}

	// Создаем бакет если его нет
	err = storage.createBucketIfNotExists()
	if err != nil {
		return nil, fmt.Errorf("failed to create bucket: %v", err)
	}

	return storage, nil
}

func (s *MinIOStorage) createBucketIfNotExists() error {
	ctx := context.Background()

	// Проверяем существует ли бакет
	exists, err := s.client.BucketExists(ctx, s.bucketName)
	if err != nil {
		return err
	}

	if !exists {
		// Создаем бакет
		err = s.client.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return err
		}

		// Устанавливаем публичную политику для чтения
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
	return fmt.Sprintf("http://%s/%s/%s", s.endpoint, s.bucketName, filename)
}