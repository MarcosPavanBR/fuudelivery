package search

import (
	"sort"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// EstablishmentResult e um resultado de busca de estabelecimento.
type EstablishmentResult struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Image          string `json:"image,omitempty"`
	LocationString string `json:"location_string,omitempty"`
	Score          int    `json:"score"`
}

// ProductResult e um resultado de busca de produto (com o estabelecimento dono).
type ProductResult struct {
	ID              uint    `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Price           float64 `json:"price"`
	Image           string  `json:"image,omitempty"`
	EstablishmentID uint    `json:"establishment_id"`
	Establishment   string  `json:"establishment_name"`
	Score           int     `json:"score"`
}

// SearchResponse e o payload de GET /search?q=...
type SearchResponse struct {
	Query          string                `json:"query"`
	Total          int                   `json:"total"`
	Establishments []EstablishmentResult `json:"establishments"`
	Products       []ProductResult       `json:"products"`
}

// establishmentRow e a projecao minima de um estabelecimento no Postgres.
type establishmentRow struct {
	ID             uint
	Name           string
	Description    string
	Image          string
	LocationString string
}

// productRow e a projecao minima de um produto (joins com estabelecimento).
type productRow struct {
	ID              uint
	Name            string
	Description     string
	Price           float64
	Image           string
	EstablishmentID uint
	Establishment   string
}

// searchEstablishments busca estabelecimentos por nome/descricao/localizacao (ILIKE).
func searchEstablishments(db *gorm.DB, query string, limit int) ([]EstablishmentResult, error) {
	like := "%" + query + "%"
	var rows []establishmentRow
	err := db.Model(&establishmentRow{}).
		Table("establishments").
		Select("id, name, description, image, location_string").
		Where("name ILIKE ? OR description ILIKE ? OR location_string ILIKE ?", like, like, like).
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	results := make([]EstablishmentResult, 0, len(rows))
	for _, r := range rows {
		results = append(results, EstablishmentResult{
			ID:             r.ID,
			Name:           r.Name,
			Description:    r.Description,
			Image:          r.Image,
			LocationString: r.LocationString,
			Score:          itemScore(query, r.Name, r.Description+" "+r.LocationString),
		})
	}
	// Ordena por relevancia (score decrescente), depois nome.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})
	return results, nil
}

// searchProducts busca produtos por nome/descricao (ILIKE) trazendo o estabelecimento.
// Nota: a tabela e aliasada como "products p" — o SELECT e o JOIN referenciam p.*.
func searchProducts(db *gorm.DB, query string, limit int) ([]ProductResult, error) {
	like := "%" + query + "%"
	var rows []productRow
	err := db.
		Table("products p").
		Select("p.id, p.name, p.description, p.price, p.image, p.establishment_id, e.name AS establishment").
		Joins("JOIN establishments e ON e.id = p.establishment_id").
		Where("p.name ILIKE ? OR p.description ILIKE ?", like, like).
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	results := make([]ProductResult, 0, len(rows))
	for _, r := range rows {
		results = append(results, ProductResult{
			ID:              r.ID,
			Name:            r.Name,
			Description:     r.Description,
			Price:           r.Price,
			Image:           r.Image,
			EstablishmentID: r.EstablishmentID,
			Establishment:   r.Establishment,
			Score:           itemScore(query, r.Name, r.Description),
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})
	return results, nil
}

// NewHandler cria o handler de GET /search.
// getDB devolve o banco principal (Postgres) com establishments e products.
//
// Recebe uma FUNÇÃO que devolve o banco, não o *gorm.DB: a rota é registrada
// no main antes de a goroutine de inicialização conectar o Postgres, então
// passar models.DB direto capturava nil e toda busca dava panic (500) para
// sempre. O banco é lido a cada requisição; sem ele, 503.
func NewHandler(getDB func() *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		db := getDB()
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Busca indisponível: banco ainda conectando",
			})
		}
		query := strings.TrimSpace(c.Query("q"))
		if !isSearchable(query) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "Informe um termo de busca com pelo menos 2 caracteres (?q=...)",
				"example": "/search?q=pizza",
			})
		}

		limit := maxResults
		establishments, err := searchEstablishments(db, query, limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Falha ao buscar estabelecimentos",
			})
		}
		products, err := searchProducts(db, query, limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Falha ao buscar produtos",
			})
		}

		total := len(establishments) + len(products)
		return c.JSON(SearchResponse{
			Query:          query,
			Total:          total,
			Establishments: establishments,
			Products:       products,
		})
	}
}
