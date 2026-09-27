package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioStorage складывает изображения и видео карточек в бакет Minio.
type MinioStorage struct {
	client *minio.Client
	bucket string
}

// NewMinioStorage подключается к Minio по переменным окружения
// MINIO_ENDPOINT, MINIO_ACCESS_KEY, MINIO_SECRET_KEY, MINIO_BUCKET.
func NewMinioStorage() (*MinioStorage, error) {
	client, err := minio.New(envOrDefault("MINIO_ENDPOINT", "localhost:9000"), &minio.Options{
		Creds: credentials.NewStaticV4(
			envOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
			envOrDefault("MINIO_SECRET_KEY", "minioadmin"),
			"",
		),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("подключение к minio: %w", err)
	}
	return &MinioStorage{client: client, bucket: bucketName()}, nil
}

// Upload кладёт файл в бакет под указанным именем и возвращает публичный url объекта.
func (s *MinioStorage) Upload(ctx context.Context, objectName string, file io.Reader, size int64, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, s.bucket, objectName, file, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("загрузка %s в minio: %w", objectName, err)
	}
	return MinioObjectURL(objectName), nil
}

// Remove удаляет объект; нужен, чтобы не оставлять в бакете файлы,
// если запись в базу не удалась.
func (s *MinioStorage) Remove(ctx context.Context, objectName string) error {
	return s.client.RemoveObject(ctx, s.bucket, objectName, minio.RemoveObjectOptions{})
}

// GenerateObjectName генерирует имя файла латиницей: исходное имя от клиента
// (оно может быть кириллическим) не используется, берётся только расширение,
// определённое по содержимому файла.
//
//	fuel-image-20260926-153012-3f9a1c2b.jpg
func GenerateObjectName(kind, extension string) string {
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	return fmt.Sprintf("fuel-%s-%s-%s%s", kind, time.Now().Format("20060102-150405"), hex.EncodeToString(suffix), extension)
}

func bucketName() string {
	return envOrDefault("MINIO_BUCKET", "heat-fuel-media")
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
