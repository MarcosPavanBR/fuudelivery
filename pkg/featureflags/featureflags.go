package featureflags

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// FeatureFlag representa uma flag de funcionalidade
type FeatureFlag struct {
	Name        string                 `json:"name"`
	Enabled     bool                   `json:"enabled"`
	RolloutPercent int                 `json:"rollout_percent"` // 0-100
	AllowedUsers []string              `json:"allowed_users"`   // Lista branca de user IDs
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
}

// FeatureFlagManager gerencia feature flags com Redis para distribuição
type FeatureFlagManager struct {
	redis      *redis.Client
	cache      sync.Map // Cache local para performance
	prefix     string
	cacheTTL   time.Duration
}

// NewFeatureFlagManager cria novo gerenciador de feature flags
func NewFeatureFlagManager(redis *redis.Client, prefix string) *FeatureFlagManager {
	return &FeatureFlagManager{
		redis:    redis,
		prefix:   prefix,
		cacheTTL: 5 * time.Second, // Cache local de 5 segundos
	}
}

// GetFlag retorna uma feature flag específica
func (m *FeatureFlagManager) GetFlag(ctx context.Context, flagName string) (*FeatureFlag, error) {
	// Tenta cache local primeiro
	if cached, ok := m.cache.Load(flagName); ok {
		if flag, valid := cached.(*FeatureFlag); valid {
			return flag, nil
		}
	}

	// Busca no Redis
	key := m.buildKey(flagName)
	data, err := m.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return &FeatureFlag{Name: flagName, Enabled: false}, nil
		}
		return nil, err
	}

	var flag FeatureFlag
	if err := json.Unmarshal(data, &flag); err != nil {
		return nil, err
	}

	// Atualiza cache local
	m.cache.Store(flagName, &flag)

	// Agenda invalidação do cache
	go func() {
		time.Sleep(m.cacheTTL)
		m.cache.Delete(flagName)
	}()

	return &flag, nil
}

// IsEnabled verifica se uma flag está habilitada para um usuário específico
func (m *FeatureFlagManager) IsEnabled(ctx context.Context, flagName, userID string) (bool, error) {
	flag, err := m.GetFlag(ctx, flagName)
	if err != nil {
		return false, err
	}

	// Flag desabilitada globalmente
	if !flag.Enabled {
		return false, nil
	}

	// Verifica expiração
	if flag.ExpiresAt != nil && time.Now().After(*flag.ExpiresAt) {
		return false, nil
	}

	// Lista branca tem prioridade
	if len(flag.AllowedUsers) > 0 {
		for _, allowed := range flag.AllowedUsers {
			if allowed == userID {
				return true, nil
			}
		}
		// Se está na lista branca mas não é este usuário, retorna false
		if userID != "" {
			return false, nil
		}
	}

	// Rollout percentual
	if flag.RolloutPercent > 0 && flag.RolloutPercent < 100 {
		return m.checkRollout(flagName, userID, flag.RolloutPercent), nil
	}

	// 100% habilitado
	return flag.RolloutPercent >= 100, nil
}

// checkRollout determina se usuário está dentro do rollout percentual
func (m *FeatureFlagManager) checkRollout(flagName, userID string, percent int) bool {
	if userID == "" {
		// Sem userID, usa hash baseado em tempo (para testes)
		return false
	}

	// Hash consistente: mesmo usuário sempre cai no mesmo resultado
	hash := hashString(flagName + ":" + userID)
	return (hash % 100) < percent
}

// EnableFlag habilita uma flag com rollout opcional
func (m *FeatureFlagManager) EnableFlag(ctx context.Context, flagName string, rolloutPercent int, allowedUsers []string) error {
	flag := &FeatureFlag{
		Name:           flagName,
		Enabled:        true,
		RolloutPercent: rolloutPercent,
		AllowedUsers:   allowedUsers,
	}

	return m.saveFlag(ctx, flag)
}

// DisableFlag desabilita uma flag
func (m *FeatureFlagManager) DisableFlag(ctx context.Context, flagName string) error {
	flag := &FeatureFlag{
		Name:    flagName,
		Enabled: false,
	}

	return m.saveFlag(ctx, flag)
}

// AddToAllowlist adiciona usuário à lista branca de uma flag
func (m *FeatureFlagManager) AddToAllowlist(ctx context.Context, flagName, userID string) error {
	flag, err := m.GetFlag(ctx, flagName)
	if err != nil {
		return err
	}

	// Verifica se já está na lista
	for _, u := range flag.AllowedUsers {
		if u == userID {
			return nil // Já está na lista
		}
	}

	flag.AllowedUsers = append(flag.AllowedUsers, userID)
	return m.saveFlag(ctx, flag)
}

// RemoveFromAllowlist remove usuário da lista branca
func (m *FeatureFlagManager) RemoveFromAllowlist(ctx context.Context, flagName, userID string) error {
	flag, err := m.GetFlag(ctx, flagName)
	if err != nil {
		return err
	}

	newList := []string{}
	for _, u := range flag.AllowedUsers {
		if u != userID {
			newList = append(newList, u)
		}
	}

	flag.AllowedUsers = newList
	return m.saveFlag(ctx, flag)
}

// SetExpiration define data de expiração para uma flag
func (m *FeatureFlagManager) SetExpiration(ctx context.Context, flagName string, expiresAt time.Time) error {
	flag, err := m.GetFlag(ctx, flagName)
	if err != nil {
		return err
	}

	flag.ExpiresAt = &expiresAt
	return m.saveFlag(ctx, flag)
}

// ListFlags retorna todas as flags cadastradas
func (m *FeatureFlagManager) ListFlags(ctx context.Context) ([]FeatureFlag, error) {
	pattern := m.prefix + ":flag:*"
	cursor := uint64(0)
	var flags []FeatureFlag

	for {
		keys, nextCursor, err := m.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			data, err := m.redis.Get(ctx, key).Bytes()
			if err != nil {
				continue
			}

			var flag FeatureFlag
			if err := json.Unmarshal(data, &flag); err != nil {
				continue
			}

			flags = append(flags, flag)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return flags, nil
}

// DeleteFlag remove uma flag
func (m *FeatureFlagManager) DeleteFlag(ctx context.Context, flagName string) error {
	key := m.buildKey(flagName)
	return m.redis.Del(ctx, key).Err()
}

// ClearCache limpa o cache local
func (m *FeatureFlagManager) ClearCache() {
	m.cache = sync.Map{}
}

func (m *FeatureFlagManager) saveFlag(ctx context.Context, flag *FeatureFlag) error {
	data, err := json.Marshal(flag)
	if err != nil {
		return err
	}

	key := m.buildKey(flag.Name)
	return m.redis.Set(ctx, key, data, 24*time.Hour).Err()
}

func (m *FeatureFlagManager) buildKey(flagName string) string {
	return m.prefix + ":flag:" + flagName
}

// hashString gera hash simples de uma string
func hashString(s string) int {
	hash := 0
	for i := 0; i < len(s); i++ {
		hash = 31*hash + int(s[i])
	}
	if hash < 0 {
		hash = -hash
	}
	return hash % 1000000
}
