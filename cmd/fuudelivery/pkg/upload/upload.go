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
	"gorm.io/gorm"
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

	if _, err := middlewares.GetUserIDFromToken(c); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
	}

	entity := c.Params("entity")
	entityID := c.Params("entityId")

	// Verifica ownership. Regras por entidade:
	//   products/categories/additionals: dono do estabelecimento da entidade
	//     (entityID é OBRIGATÓRIO — sem ID não há a quem vincular o arquivo);
	//   restaurants: dono do próprio estabelecimento;
	//   reviews/avatars: qualquer autenticado (conteúdo do próprio usuário).
	if entity == "products" || entity == "categories" || entity == "additionals" {
		if entityID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "entityId é obrigatório para " + entity})
		}
		if !canUploadFor(c, entity, entityID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You can only upload images for your own establishment"})
		}
	}
	if entity == "restaurants" {
		role, _ := middlewares.GetUserRoleFromToken(c)
		tokenEstID, estErr := middlewares.GetEstablishmentIDFromToken(c)
		if role != "admin" && (estErr != nil || tokenEstID <= 0) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Sem estabelecimento vinculado ao token"})
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

// canUploadFor decide se o token pode enviar imagem para a entidade
// (products/categories/additionals). Admin passa; os demais só para
// entidades do establishment_id DO TOKEN — a mesma regra de
// canActOnEstablishment.
//
// Antes o id do token era buscado na tabela users. Clientes, usuários de loja
// e entregadores têm sequências de id independentes: o cliente de id 5
// herdava a loja do usuário 5 e enviava imagens para os produtos dela.
func canUploadFor(c *fiber.Ctx, entity, entityID string) bool {
	if role, _ := middlewares.GetUserRoleFromToken(c); role == "admin" {
		return true
	}
	estID, err := middlewares.GetEstablishmentIDFromToken(c)
	if err != nil || estID <= 0 {
		return false
	}
	return entityBelongsTo(models.DB, entity, entityID, estID)
}

// entityBelongsTo confere se a entidade é do estabelecimento. As três
// tabelas têm establishment_id (additionals também: o JOIN antigo por
// additionals.product_id usava uma coluna que não existe, e o upload de
// adicional dava 403 para todo mundo).
func entityBelongsTo(db *gorm.DB, entity, entityID string, establishmentID int64) bool {
	table := map[string]string{
		"products":    "products",
		"categories":  "categories",
		"additionals": "additionals",
	}[entity]
	if table == "" || db == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var count int64
	if err := db.WithContext(ctx).Table(table).
		Where("id = ? AND establishment_id = ?", entityID, establishmentID).
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
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
