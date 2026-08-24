// Package audit registra acoes administrativas (quem fez o que, quando e de
// onde) numa tabela dedicada admin_audit_log no Postgres. Diferente do
// audit_log por trigger (05_audit_log.sql — changelog de DADOS), este log e
// SEMANTICO: guarda a acao de painel (aprovou pagamento, excluiu usuario...)
// com a identidade do admin vinda do JWT.
//
// Regra de ouro: gravar auditoria NUNCA pode falhar a requisicao do admin.
// Se o DB nao estiver disponivel, a acao e logada apenas no stdout do servidor.
package audit

import (
	"encoding/json"
	"log"
	"time"

	"github.com/carloshomar/fuudelivery/auth_api/app/middlewares"
	"github.com/carloshomar/fuudelivery/auth_api/app/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// AdminAuditLog e o modelo da tabela admin_audit_log.
type AdminAuditLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	AdminUserID  uint      `gorm:"index" json:"admin_user_id"`
	AdminName    string    `json:"admin_name"`
	AdminEmail   string    `json:"admin_email"`
	Action       string    `gorm:"index" json:"action"`
	ResourceType string    `gorm:"index" json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	Details      string    `json:"details"` // JSON opaco com contexto da acao
	IP           string    `json:"ip"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

// Init garante que a tabela exista. Chamado no startup do monolito, apos o
// ConnectDatabase. Idempotente; nunca trava o boot se falhar.
func Init(db *gorm.DB) {
	if db == nil {
		return
	}
	if err := db.AutoMigrate(&AdminAuditLog{}); err != nil {
		log.Printf("[AUDIT] Falha no AutoMigrate de admin_audit_log: %v", err)
	}
}

// actorFromCtx extrai id/nome/email do admin autenticado. Nome e email sao
// lidos do Postgres (tabela users); se indisponivel, retorna so o id.
func actorFromCtx(c *fiber.Ctx) (uint, string, string) {
	uid, err := middlewares.GetUserIDFromToken(c)
	if err != nil || uid <= 0 {
		return 0, "", ""
	}
	id := uint(uid)
	if models.DB == nil {
		return id, "", ""
	}
	var u struct {
		Name  string
		Email string
	}
	if err := models.DB.Table("users").Select("name, email").Where("id = ?", id).First(&u).Error; err != nil {
		return id, "", ""
	}
	return id, u.Name, u.Email
}

// Record grava uma acao administrativa. Nunca falha a requisicao: se o db for
// nil (ex.: testes sem Postgres) ou a gravacao falhar, apenas loga.
func Record(c *fiber.Ctx, db *gorm.DB, action, resourceType, resourceID string, details map[string]interface{}) {
	if db == nil || c == nil {
		return
	}
	adminID, name, email := actorFromCtx(c)

	var detailsJSON string
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			detailsJSON = string(b)
		}
	}

	entry := AdminAuditLog{
		AdminUserID:  adminID,
		AdminName:    name,
		AdminEmail:   email,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      detailsJSON,
		IP:           c.IP(),
	}
	if err := db.Create(&entry).Error; err != nil {
		log.Printf("[AUDIT] Falha ao gravar %s (%s %s): %v", action, resourceType, resourceID, err)
	}
}

// Query e o filtro de listagem do log de auditoria.
type Query struct {
	Action       string
	Admin        string // busca parcial por nome ou email do admin
	ResourceType string
	From, To     *time.Time
	Page, Limit  int
}

// List retorna as entradas filtradas (mais recentes primeiro) e o total.
func List(db *gorm.DB, q Query) ([]AdminAuditLog, int64, error) {
	tx := db.Model(&AdminAuditLog{})
	if q.Action != "" {
		tx = tx.Where("action = ?", q.Action)
	}
	if q.Admin != "" {
		like := "%" + q.Admin + "%"
		tx = tx.Where("(admin_name ILIKE ? OR admin_email ILIKE ?)", like, like)
	}
	if q.ResourceType != "" {
		tx = tx.Where("resource_type = ?", q.ResourceType)
	}
	if q.From != nil {
		tx = tx.Where("created_at >= ?", *q.From)
	}
	if q.To != nil {
		tx = tx.Where("created_at <= ?", *q.To)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := q.Page
	if page < 1 {
		page = 1
	}
	limit := q.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	var entries []AdminAuditLog
	if err := tx.Order("created_at DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&entries).Error; err != nil {
		return nil, 0, err
	}
	if entries == nil {
		entries = []AdminAuditLog{}
	}
	return entries, total, nil
}
