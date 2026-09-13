package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/carloshomar/fuudelivery/pkg/secretbox"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================================
// Recipient — a conta de gateway conectada de um recebedor (split na origem).
//
// Mapeia a tabela `recipients` (sql/14 + sql/25). Cada estabelecimento conecta
// a PRÓPRIA conta do gateway (Mercado Pago, via OAuth) e a fatia da venda dele
// cai DIRETO na conta dele — o dinheiro nunca passa pela plataforma. Este
// modelo guarda o vínculo e os tokens que autorizam a cobrança em nome do
// lojista.
//
// Os tokens ficam SEMPRE cifrados (AES-256-GCM, pkg/secretbox) nas colunas
// *_enc. Nunca em texto puro, nunca em JSON de resposta (`json:"-"`). Quem tem
// o token cobra em nome do lojista — é a credencial mais sensível do sistema.
// ============================================================================

// ErrNoActiveRecipient é devolvido quando não há recebedor ATIVO para o
// gateway/usuário pedido. O caminho de cobrança traduz isto em 409 e BLOQUEIA a
// venda: a plataforma não segura dinheiro que não sabe rotear.
var ErrNoActiveRecipient = errors.New("nenhum recebedor ativo para este gateway")

// Estados válidos (espelham o CHECK chk_recipients_status de sql/14).
const (
	RecipientPending = "pending"
	RecipientActive  = "active"
	RecipientBlocked = "blocked"
)

// Recipient — linha da tabela `recipients`.
type Recipient struct {
	ID                 int64  `gorm:"primaryKey;column:id" json:"id"`
	UserType           string `gorm:"column:user_type" json:"user_type"` // establishment | delivery_man | restaurant
	UserID             int64  `gorm:"column:user_id" json:"user_id"`
	Gateway            string `gorm:"column:gateway" json:"gateway"` // mercadopago | pagarme | asaas | abacatepay
	GatewayRecipientID string `gorm:"column:gateway_recipient_id" json:"gateway_recipient_id"`
	Status             string `gorm:"column:status" json:"status"`
	MPUserID           string `gorm:"column:mp_user_id" json:"mp_user_id,omitempty"`

	// Tokens cifrados. json:"-" para NUNCA vazarem numa resposta de API.
	AccessTokenEnc  []byte     `gorm:"column:access_token_enc" json:"-"`
	RefreshTokenEnc []byte     `gorm:"column:refresh_token_enc" json:"-"`
	TokenExpiresAt  *time.Time `gorm:"column:token_expires_at" json:"token_expires_at,omitempty"`

	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName fixa o nome da tabela.
func (Recipient) TableName() string { return "recipients" }

// SetTokens cifra e grava os tokens no struct (não persiste — quem chama faz o
// Save/Upsert). Centralizar aqui garante que ninguém grave token em claro por
// esquecimento.
func (r *Recipient) SetTokens(box *secretbox.Box, accessToken, refreshToken string, expiresAt time.Time) error {
	if box == nil {
		return fmt.Errorf("recipient: secretbox nil — não há como cifrar o token")
	}
	acc, err := box.SealString(accessToken)
	if err != nil {
		return fmt.Errorf("cifrar access_token: %w", err)
	}
	ref, err := box.SealString(refreshToken)
	if err != nil {
		return fmt.Errorf("cifrar refresh_token: %w", err)
	}
	r.AccessTokenEnc = acc
	r.RefreshTokenEnc = ref
	r.TokenExpiresAt = &expiresAt
	return nil
}

// AccessToken decifra e devolve o access_token em claro, só na hora de usar
// (criar a cobrança). O valor não deve ser logado nem persistido.
func (r *Recipient) AccessToken(box *secretbox.Box) (string, error) {
	if box == nil {
		return "", fmt.Errorf("recipient: secretbox nil — não há como decifrar o token")
	}
	if len(r.AccessTokenEnc) == 0 {
		return "", fmt.Errorf("recipient %d: sem access_token gravado", r.ID)
	}
	return box.OpenString(r.AccessTokenEnc)
}

// RefreshToken decifra o refresh_token (usado pelo job de refresh).
func (r *Recipient) RefreshToken(box *secretbox.Box) (string, error) {
	if box == nil {
		return "", fmt.Errorf("recipient: secretbox nil — não há como decifrar o token")
	}
	if len(r.RefreshTokenEnc) == 0 {
		return "", fmt.Errorf("recipient %d: sem refresh_token gravado", r.ID)
	}
	return box.OpenString(r.RefreshTokenEnc)
}

// ActiveRecipient busca o recebedor ATIVO de um usuário num gateway. Devolve
// ErrNoActiveRecipient quando não há — o caminho de cobrança bloqueia a venda
// nesse caso.
func ActiveRecipient(db *gorm.DB, gateway, userType string, userID int64) (*Recipient, error) {
	var r Recipient
	err := db.Where("gateway = ? AND user_type = ? AND user_id = ? AND status = ?",
		gateway, userType, userID, RecipientActive).
		First(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNoActiveRecipient
	}
	if err != nil {
		return nil, fmt.Errorf("buscar recebedor ativo (%s/%s/%d): %w", gateway, userType, userID, err)
	}
	return &r, nil
}

// UpsertRecipient insere ou atualiza o recebedor pela chave natural
// (user_type, user_id, gateway) — a mesma do UNIQUE uq_recipients_user_gateway.
// Usado no callback do OAuth: reconectar a conta sobrescreve os tokens em vez
// de criar linha duplicada.
func UpsertRecipient(db *gorm.DB, r *Recipient) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	r.UpdatedAt = time.Now()
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_type"}, {Name: "user_id"}, {Name: "gateway"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"gateway_recipient_id", "status", "mp_user_id",
			"access_token_enc", "refresh_token_enc", "token_expires_at", "updated_at",
		}),
	}).Create(r).Error
}
