// Package storage fornece integracao com Supabase Storage (S3-compatible).
// Usa a API REST do Supabase Storage para upload/download de arquivos
// de imagens de restaurantes, produtos e categorias.
package storage

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SupabaseStorage gerencia upload/download no Supabase Storage.
type SupabaseStorage struct {
	BaseURL    string // SUPABASE_URL (ex: https://xxxxx.supabase.co)
	ServiceKey string // SUPABASE_SERVICE_ROLE_KEY
	BucketName string // Nome do bucket (ex: fuudelivery-images)
}

// NewSupabaseStorage cria uma nova instancia do storage.
// Se as env vars nao estiverem configuradas, retorna nil (desativado).
func NewSupabaseStorage() *SupabaseStorage {
	baseURL := os.Getenv("SUPABASE_URL")
	serviceKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

	if baseURL == "" || serviceKey == "" {
		log.Println("[STORAGE] SUPABASE_URL ou SUPABASE_SERVICE_ROLE_KEY nao configurados. Storage desativado.")
		return nil
	}

	bucket := os.Getenv("STORAGE_BUCKET")
	if bucket == "" {
		bucket = "fuudelivery-images"
	}

	return &SupabaseStorage{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		ServiceKey: serviceKey,
		BucketName: bucket,
	}
}

// Upload faz upload de um arquivo para o Supabase Storage.
// Retorna a URL publica do arquivo.
// path: caminho dentro do bucket (ex: "restaurants/123/logo.jpg")
func (s *SupabaseStorage) Upload(path string, data []byte, contentType string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("storage nao configurado")
	}

	uploadURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.BaseURL, s.BucketName, path)

	req, err := http.NewRequest("POST", uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("erro ao criar request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.ServiceKey)
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao fazer upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload falhou (status %d): %s", resp.StatusCode, string(body))
	}

	// URL publica do arquivo
	publicURL := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.BaseURL, s.BucketName, path)
	log.Printf("[STORAGE] Upload concluido: %s", publicURL)

	return publicURL, nil
}

// Delete remove um arquivo do Supabase Storage.
func (s *SupabaseStorage) Delete(path string) error {
	if s == nil {
		return fmt.Errorf("storage nao configurado")
	}

	deleteURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.BaseURL, s.BucketName, path)

	req, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		return fmt.Errorf("erro ao criar request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.ServiceKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erro ao deletar arquivo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete falhou (status %d): %s", resp.StatusCode, string(body))
	}

	log.Printf("[STORAGE] Arquivo deletado: %s", path)
	return nil
}

// GetPublicURL retorna a URL publica de um arquivo no bucket.
func (s *SupabaseStorage) GetPublicURL(path string) string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.BaseURL, s.BucketName, path)
}

// AllowedExtensions lista as extensoes permitidas para upload.
var AllowedExtensions = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

// ValidateImageFile valida o tipo e tamanho do arquivo de imagem.
// maxSize: tamanho maximo em bytes (default 5MB).
func ValidateImageFile(filename string, data []byte, maxSize int64) error {
	if maxSize <= 0 {
		maxSize = 5 * 1024 * 1024 // 5MB default
	}

	if int64(len(data)) > maxSize {
		return fmt.Errorf("arquivo muito grande. Maximo: %dMB", maxSize/(1024*1024))
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if _, ok := AllowedExtensions[ext]; !ok {
		return fmt.Errorf("extensao nao permitida: %s. Permitidas: jpg, jpeg, png, gif, webp, svg", ext)
	}

	return nil
}

// GenerateFilePath gera um caminho unico para o arquivo no storage.
// Ex: "products/123/abc123.jpg"
func GenerateFilePath(folder string, entityID uint, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	timestamp := time.Now().UnixMilli()
	return fmt.Sprintf("%s/%d/%d%s", folder, entityID, timestamp, ext)
}
