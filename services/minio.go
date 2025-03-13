package services

import (
	"context"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioService struct {
	Client       *minio.Client
	PublicClient *minio.Client
}

// GetPresignedURL generates a presigned URL for the given bucket and object name.
func (s *MinioService) GetPresignedURL(bucketName, objectPath string, expiry time.Duration) (string, error) {
	ctx := context.Background()
	reqParams := make(url.Values)
	presignedURL, err := s.Client.PresignedGetObject(ctx, bucketName, objectPath, expiry, reqParams)
	if err != nil {
		print("Error in GetPresignedURL")
		return "", err
	}

	log.Printf("Generated presigned URL: %s", presignedURL.String())
	minioPublicURL := os.Getenv("MINIO_PUBLIC_URL")

	presignedURL.Host = minioPublicURL
	presignedURL.Scheme = "http"

	return presignedURL.String(), nil
}

func NewMinioService(client *minio.Client, publicClient *minio.Client) *MinioService {
	return &MinioService{
		Client:       client,
		PublicClient: publicClient,
	}
}

func ConnectToMinio() (*MinioService, error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	publicURL := os.Getenv("MINIO_PUBLIC_URL")
	accessKeyID := os.Getenv("MINIO_ACCESS_KEY")
	secretAccessKey := os.Getenv("MINIO_SECRET_KEY")

	log.Println(endpoint)
	log.Println(accessKeyID)
	log.Println(secretAccessKey)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})

	minioPublicClient, err := minio.New(publicURL, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})

	log.Println("TRYING TO CONNECT")
	if err != nil {
		log.Println("FAILED TO CONNECT TO MINIO ")
		log.Println("================================")
		log.Println("================================")
		log.Println("================================")

		log.Println(err)
		return nil, err
	}

	log.Println("Connected to Minio")
	log.Printf("%#v\n", minioClient)
	return NewMinioService(minioClient, minioPublicClient), nil
}
