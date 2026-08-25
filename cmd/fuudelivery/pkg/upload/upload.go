// Package upload fornece handlers HTTP para upload de imagens.
package upload

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/carloshomar/fuudelivery/pkg/storage"
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

	userID, err := middlewares.GetUserIDFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	entity := c.Params("entity")
	entityID := c.Params("entityId")

	// Verifica ownership: usuario so pode fazer upload para entidades do seu restaurante
	if entity == "products" || entity == "categories" || entity == "additionals" {
		if entityID != "" {
			if !checkOwnership(userID, entity, entityID) {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You can only upload images for your own establishment"})
			}
		}
	}

	if entity == "" {
		return c.Status(400).JSON(fiber.Map{"error": "entity is required (products, categories, restaurants, additionals)"})
	}

	// Valida entidade
	validEntities := map[string]string{
		"products":    "products",
		"categories":  "categories",
		"restaurants": "restaurants",
		"additionals": "additionals",
		"reviews":     "reviews",
		"avatars":     "avatars",
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

	// Valida tipo por CONTEÚDO (magic bytes via http.DetectContentType) —
	// o Content-Type do multipart é definido pelo cliente e pode ser forjado
	// (ex.: HTML/SVG com JS mandado como "image/png").
	sniffed := http.DetectContentType(data)
	if !strings.HasPrefix(sniffed, "image/") {
		return c.Status(400).JSON(fiber.Map{"error": "Conteúdo não é uma imagem válida"})
	}
	// SVG nunca passa: pode embutir <script> (XSS armazenado no domínio do storage).
	if strings.Contains(sniffed, "image/svg") || strings.HasSuffix(strings.ToLower(file.Filename), ".svg") {
		return c.Status(400).JSON(fiber.Map{"error": "SVG não permitido"})
	}
	contentType := sniffed

	if len(data) > 5*1024*1024 {
		return c.Status(400).JSON(fiber.Map{"error": "Arquivo muito grande. Maximo: 5MB"})
	}

	// Gera caminho unico
	path := storage.GenerateFilePath(folder, 0, file.Filename)
	if entityID != "" {
		var id uint
		if err := parseUint(entityID, &id); err == nil && id > 0 {
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

// checkOwnership verifica se o usuario autenticado e dono da entidade.
// Para products/categories/additionals, verifica se pertencem ao establishment do usuario.
func checkOwnership(userID int64, entity, entityID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var user models.User
	if err := models.DB.WithContext(ctx).First(&user, userID).Error; err != nil {
		return false
	}

	if user.EstablishmentID == 0 {
		return false // usuario sem restaurante vinculado
	}

	switch entity {
	case "products":
		var count int64
		models.DB.WithContext(ctx).Table("products").
			Where("id = ? AND establishment_id = ?", entityID, user.EstablishmentID).
			Count(&count)
		return count > 0
	case "categories":
		var count int64
		models.DB.WithContext(ctx).Table("categories").
			Where("id = ? AND establishment_id = ?", entityID, user.EstablishmentID).
			Count(&count)
		return count > 0
	case "additionals":
		// Additionals estao vinculados via product -> establishment
		var count int64
		models.DB.WithContext(ctx).Table("additionals").
			Joins("JOIN products ON products.id = additionals.product_id").
			Where("additionals.id = ? AND products.establishment_id = ?", entityID, user.EstablishmentID).
			Count(&count)
		return count > 0
	default:
		return true // restaurants/reviews: sem check por enquanto
	}
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
