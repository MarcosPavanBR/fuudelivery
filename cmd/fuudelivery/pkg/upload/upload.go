// Package upload fornece handlers HTTP para upload de imagens.
package upload

import (
	"io"
	"log"
	"strings"

	"github.com/carloshomar/fuudelivery/pkg/storage"
	"github.com/carloshomar/vercardapio/auth_api/app/middlewares"
	"github.com/gofiber/fiber/v2"
)

var store *storage.SupabaseStorage

// Init inicializa o storage (chamado na inicializacao do servidor).
func Init() {
	store = storage.NewSupabaseStorage()
	if store == nil {
		log.Println("[UPLOAD] Supabase Storage nao configurado. Upload de imagens desativado.")
	}
}

// HandleImageUpload processa upload de imagem para uma entidade.
// Uso: POST /upload/:entity/:entityId
// entity: "products", "categories", "restaurants", "additionals"
// entityId: ID da entidade (opcional)
// Multipart form: "file" = arquivo de imagem
func HandleImageUpload(c *fiber.Ctx) error {
	if store == nil {
		return c.Status(503).JSON(fiber.Map{"error": "Storage nao configurado. Configure SUPABASE_URL e SUPABASE_SERVICE_ROLE_KEY."})
	}

	if _, err := middlewares.ValidateJWT(c); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	entity := c.Params("entity")
	entityID := c.Params("entityId")

	if entity == "" {
		return c.Status(400).JSON(fiber.Map{"error": "entity is required (products, categories, restaurants, additionals)"})
	}

	// Valida entidade
	validEntities := map[string]string{
		"products":     "products",
		"categories":   "categories",
		"restaurants":  "restaurants",
		"additionals":  "additionals",
		"reviews":      "reviews",
	}
	folder, ok := validEntities[entity]
	if !ok {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid entity. Use: products, categories, restaurants, additionals, reviews"})
	}

	// Le o arquivo do multipart form
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Arquivo nao enviado. Use campo \"file\" no multipart form."})
	}

	// Abre o arquivo
	f, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao abrir arquivo"})
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Erro ao ler arquivo"})
	}

	// Valida tipo e tamanho
	contentType := file.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return c.Status(400).JSON(fiber.Map{"error": "Apenas imagens sao permitidas (image/*)"})
	}

	if len(data) > 5*1024*1024 {
		return c.Status(400).JSON(fiber.Map{"error": "Arquivo muito grande. Maximo: 5MB"})
	}

	// Gera caminho unico
	path := storage.GenerateFilePath(folder, 0, file.Filename)
	if entityID != "" {
		var id uint
		if _, err := parseUint(entityID, &id); err == nil && id > 0 {
			path = storage.GenerateFilePath(folder, id, file.Filename)
		}
	}

	// Upload para Supabase Storage
	publicURL, err := store.Upload(path, data, contentType)
	if err != nil {
		log.Printf("[UPLOAD] Erro ao fazer upload: %v", err)
		return c.Status(502).JSON(fiber.Map{"error": "Falha ao fazer upload da imagem"})
	}

	return c.JSON(fiber.Map{
		"url":  publicURL,
		"path": path,
	})
}

// parseUint helper para converter string para uint.
func parseUint(s string, out *uint) error {
	var v uint
	for _, c := range s {
		if c < '0' || c > '9' {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid ID")
		}
		v = v*10 + uint(c-'0')
	}
	*out = v
	return nil
}
