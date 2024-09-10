package services

import (
	"log"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioService struct {
	Client *minio.Client
}

func NewMinioService(client *minio.Client) *MinioService {
	return &MinioService{
		Client: client,
	}
}

func ConnectToMinio() (*MinioService, error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKeyID := os.Getenv("MINIO_ACCESS_KEY")
	secretAccessKey := os.Getenv("MINIO_SECRET_KEY")

	useSSL := false

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})

	if err != nil {
		log.Println(err)
		return nil, err
	}

	log.Println("Connected to Minio")

	return NewMinioService(minioClient), nil
}
