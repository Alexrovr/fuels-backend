package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// respondError — единый формат ошибки веб-сервиса: {"error": "..."}.
func respondError(ctx *gin.Context, status int, message string) {
	ctx.JSON(status, gin.H{"error": message})
}

// parseFuelID читает :fuel_id из url.
func parseFuelID(ctx *gin.Context) (uint, bool) {
	fuelID, err := strconv.ParseUint(ctx.Param("fuel_id"), 10, 64)
	if err != nil || fuelID == 0 {
		respondError(ctx, http.StatusBadRequest, "Идентификатор вида топлива — целое положительное число")
		return 0, false
	}
	return uint(fuelID), true
}

const systemFieldsHint = "системные поля (ид, статус, создатель, даты создания и формирования) " +
	"вычисляются на бэкенде и с клиента не принимаются"

// decodeStrictJSON разбирает тело запроса в сериализатор. Неизвестные поля —
// в том числе попытка передать fuel_status, creator_id, formed_at — дают ошибку.
// Пустое тело допустимо: тогда все поля остаются незаполненными.
func decodeStrictJSON(ctx *gin.Context, dst any) error {
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()

	err := decoder.Decode(dst)
	switch {
	case err == nil, errors.Is(err, io.EOF):
		return nil
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return fmt.Errorf("поле %s не принимается: %s", field, systemFieldsHint)
	default:
		return fmt.Errorf("некорректный JSON в теле запроса: %v", err)
	}
}

// --- Файлы ---------------------------------------------------------------- //

type mediaRule struct {
	field      string // имя поля формы
	kind       string // часть генерируемого имени файла
	title      string // для сообщений об ошибках
	maxSize    int64
	extensions map[string]string // content-type -> расширение
}

var imageRule = mediaRule{
	field:   "image",
	kind:    "image",
	title:   "изображение",
	maxSize: 5 << 20,
	extensions: map[string]string{
		"image/jpeg":    ".jpg",
		"image/png":     ".png",
		"image/gif":     ".gif",
		"image/webp":    ".webp",
		"image/svg+xml": ".svg",
	},
}

var videoRule = mediaRule{
	field:   "video",
	kind:    "video",
	title:   "короткое видео",
	maxSize: 30 << 20,
	extensions: map[string]string{
		"video/mp4":  ".mp4",
		"video/webm": ".webm",
	},
}

// maxUploadBody — предел всего тела запроса с изображением и видео.
const maxUploadBody = 36 << 20

type checkedMedia struct {
	header      *multipart.FileHeader
	contentType string
	extension   string
}

// checkMedia проверяет обязательный файл формы: размер и тип по содержимому
// (первые 512 байт), а не по расширению и заголовку от клиента.
func checkMedia(form *multipart.Form, rule mediaRule) (checkedMedia, error) {
	headers := form.File[rule.field]
	if len(headers) == 0 {
		return checkedMedia{}, fmt.Errorf("не передан файл %q (%s)", rule.field, rule.title)
	}
	if len(headers) > 1 {
		return checkedMedia{}, fmt.Errorf("в поле %q ожидается один файл", rule.field)
	}
	header := headers[0]

	if header.Size == 0 {
		return checkedMedia{}, fmt.Errorf("файл %q пустой", rule.field)
	}
	if header.Size > rule.maxSize {
		return checkedMedia{}, fmt.Errorf("%s больше %d МБ", rule.title, rule.maxSize>>20)
	}

	file, err := header.Open()
	if err != nil {
		return checkedMedia{}, fmt.Errorf("не удалось открыть файл %q", rule.field)
	}
	defer file.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return checkedMedia{}, fmt.Errorf("не удалось прочитать файл %q", rule.field)
	}
	head = head[:n]

	contentType := sniffContentType(head)
	extension, ok := rule.extensions[contentType]
	if !ok {
		return checkedMedia{}, fmt.Errorf("%q: тип %s не подходит, ожидается %s", rule.field, contentType, rule.title)
	}

	return checkedMedia{header: header, contentType: contentType, extension: extension}, nil
}

func sniffContentType(head []byte) string {
	contentType := http.DetectContentType(head)
	// SVG стандартная библиотека определяет как text/xml или text/plain.
	if strings.HasPrefix(contentType, "text/") && bytes.Contains(bytes.ToLower(head), []byte("<svg")) {
		return "image/svg+xml"
	}
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = contentType[:i]
	}
	return contentType
}

// absoluteMediaURL: медиа по умолчанию хранятся относительным путём
// (/resources/media/...), а SPA нужен полный адрес на этом сервере.
func absoluteMediaURL(ctx *gin.Context) func(string) string {
	return func(raw string) string {
		if !strings.HasPrefix(raw, "/") {
			return raw
		}
		scheme := "http"
		if ctx.Request.TLS != nil {
			scheme = "https"
		}
		return scheme + "://" + ctx.Request.Host + raw
	}
}
