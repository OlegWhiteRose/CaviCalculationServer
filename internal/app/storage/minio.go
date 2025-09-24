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
	minioEndpoint := "localhost:9000"
	nginxEndpoint := "localhost:8080"
	accessKeyID := "admin"
	secretAccessKey := "password123"
	bucketName := "images"


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