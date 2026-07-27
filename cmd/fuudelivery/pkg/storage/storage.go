// Package storage fornece integracao com Supabase Storage para upload de imagens.
package storage

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SupabaseStorage gerencia uploads de imagens para o Supabase Storage.
type SupabaseStorage struct {
	URL            string // URL do projeto Supabase (SUPABASE_URL)
	ServiceRoleKey string // Chave de servico (SUPABASE_SERVICE_ROLE_KEY)
	Bucket         string // Nome do bucket (padrao: "fuudelivery")
}

// NewSupabaseStorage cria uma nova instancia lendo variaveis de ambiente.
// Retorna nil se SUPABASE_URL ou SUPABASE_SERVICE_ROLE_KEY nao estiverem configurados.
func NewSupabaseStorage() *SupabaseStorage {
	url := os.Getenv("SUPABASE_URL")
	key := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	if url == "" || key == "" {
		return nil
	}
	bucket := os.Getenv("SUPABASE_STORAGE_BUCKET")
	if bucket == "" {
		bucket = "fuudelivery"
	}
	return &SupabaseStorage{
		URL:            strings.TrimRight(url, "/"),
		ServiceRoleKey: key,
		Bucket:         bucket,
	}
}

// GenerateFilePath gera um caminho unico para o arquivo no bucket.
// padrao: {folder}/{timestamp}_{filename}
func GenerateFilePath(folder string, entityID uint, filename string) string {
	cleanName := strings.ReplaceAll(filename, " ", "_")
	cleanName = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, cleanName)

	ts := time.Now().Format("20060102_150405")

	if entityID > 0 {
		return fmt.Sprintf("%s/%d_%s_%s", folder, entityID, ts, cleanName)
	}
	return fmt.Sprintf("%s/%s_%s", folder, ts, cleanName)
}

// Upload faz upload de bytes para o Supabase Storage e retorna a URL publica.
func (s *SupabaseStorage) Upload(path string, data []byte, contentType string) (string, error) {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.URL, s.Bucket, path)

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("erro ao criar requisicao: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.ServiceRoleKey)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "true")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao enviar arquivo: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return "", fmt.Errorf("supabase retornou %d: %s", resp.StatusCode, string(body))
	}

	publicURL := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.URL, s.Bucket, path)
	return publicURL, nil
}

// Delete remove um arquivo do Supabase Storage.
func (s *SupabaseStorage) Delete(path string) error {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.URL, s.Bucket, path)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.ServiceRoleKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase retornou %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetExtension extrai a extensao de um nome de arquivo.
func GetExtension(filename string) string {
	return filepath.Ext(filename)
}

// ParseMultipartForm helper para extrair arquivo de multipart.
func ParseMultipartForm(file *multipart.FileHeader) ([]byte, error) {
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
