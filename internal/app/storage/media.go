package storage

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Медиа по умолчанию лежат на самом SSR-сервере рядом со стилями и иконками.
const (
	DefaultImagePath = "/resources/media/fuel_default.svg"
	DefaultVideoPath = "/resources/media/fuel_default.mp4"
)

const (
	availabilityTTL     = 30 * time.Second
	availabilityTimeout = 700 * time.Millisecond
)

type availability struct {
	ok        bool
	checkedAt time.Time
}

// MediaResolver отдаёт шаблонам адрес медиафайла карточки: либо url из базы,
// либо файл по умолчанию, если поле пустое или объект недоступен.
type MediaResolver struct {
	client *http.Client

	mu    sync.Mutex
	cache map[string]availability
}

func NewMediaResolver() *MediaResolver {
	return &MediaResolver{
		client: &http.Client{Timeout: availabilityTimeout},
		cache:  make(map[string]availability),
	}
}

func (m *MediaResolver) ImageURL(rawURL string) string {
	return m.resolve(rawURL, DefaultImagePath)
}

func (m *MediaResolver) VideoURL(rawURL string) string {
	return m.resolve(rawURL, DefaultVideoPath)
}

func (m *MediaResolver) resolve(rawURL, defaultPath string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || !m.available(rawURL) {
		return defaultPath
	}
	return rawURL
}

// available проверяет объект запросом HEAD. Результат кэшируется, иначе на
// каждую отрисовку плитки уходило бы по запросу на карточку.
func (m *MediaResolver) available(rawURL string) bool {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return true
	}

	m.mu.Lock()
	cached, found := m.cache[rawURL]
	m.mu.Unlock()
	if found && time.Since(cached.checkedAt) < availabilityTTL {
		return cached.ok
	}

	response, err := m.client.Head(rawURL)
	ok := err == nil && response.StatusCode == http.StatusOK
	if response != nil {
		response.Body.Close()
	}

	m.mu.Lock()
	m.cache[rawURL] = availability{ok: ok, checkedAt: time.Now()}
	m.mu.Unlock()

	return ok
}

// MinioObjectURL собирает публичный адрес объекта в бакете Minio.
// Используется при первичном наполнении базы.
func MinioObjectURL(objectKey string) string {
	if objectKey == "" {
		return ""
	}

	endpoint := os.Getenv("MINIO_PUBLIC_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:9000"
	}
	bucket := os.Getenv("MINIO_BUCKET")
	if bucket == "" {
		bucket = "heat-fuel-media"
	}

	return fmt.Sprintf("%s/%s/%s", strings.TrimRight(endpoint, "/"), bucket, objectKey)
}
