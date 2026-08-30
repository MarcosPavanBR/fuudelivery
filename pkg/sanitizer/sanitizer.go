package sanitizer

import (
	"regexp"
	"strings"
)

// Dados sensíveis que devem ser mascarados em logs
var (
	// CPF: 000.000.000-00 ou 00000000000
	cpfRegex = regexp.MustCompile(`\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b`)
	
	// CNPJ: 00.000.000/0000-00 ou 00000000000000
	cnpjRegex = regexp.MustCompile(`\b\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2}\b`)
	
	// Cartão de crédito: 16 dígitos com ou sem separadores
	cardRegex = regexp.MustCompile(`\b(?:\d{4}[- ]?){3}\d{4}\b`)
	
	// Telefone: (00) 00000-0000 ou 00000000000
	phoneRegex = regexp.MustCompile(`\b\(?\d{2}\)?[- ]?\d{4,5}[- ]?\d{4}\b`)
	
	// Email
	emailRegex = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
	
	// Chaves API (padrões comuns)
	apiKeyRegex = regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret[_-]?key|access[_-]?token)[\"']?\s*[:=]\s*[\"']?[A-Za-z0-9_-]{16,}`)
	
	// Connection strings
	connectionStringRegex = regexp.MustCompile(`(?i)(postgres|mongodb|redis|mysql)://[^:\s]+:[^@\s]+@`)
)

// LogSanitizer remove ou mascara dados sensíveis de strings antes de logar
type LogSanitizer struct {
	maskChar   string
	showLast   int
	showFirst  int
}

// NewLogSanitizer cria nova instância com configurações padrão
func NewLogSanitizer() *LogSanitizer {
	return &LogSanitizer{
		maskChar:  "*",
		showLast:  4,
		showFirst: 0,
	}
}

// Sanitize remove dados sensíveis de uma string
func (s *LogSanitizer) Sanitize(input string) string {
	if input == "" {
		return input
	}

	result := input

	// Mascara CPF
	result = cpfRegex.ReplaceAllStringFunc(result, func(match string) string {
		return s.maskString(match)
	})

	// Mascara CNPJ
	result = cnpjRegex.ReplaceAllStringFunc(result, func(match string) string {
		return s.maskString(match)
	})

	// Mascara cartão de crédito (mostra apenas últimos 4 dígitos)
	result = cardRegex.ReplaceAllStringFunc(result, func(match string) string {
		digits := regexp.MustCompile(`\d`).FindAllString(match, -1)
		if len(digits) >= 4 {
			lastFour := strings.Join(digits[len(digits)-4:], "")
			return "****-****-****-" + lastFour
		}
		return s.maskString(match)
	})

	// Mascara telefone
	result = phoneRegex.ReplaceAllStringFunc(result, func(match string) string {
		return s.maskString(match)
	})

	// Mascara email (mantém primeiras letras e domínio)
	result = emailRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := strings.Split(match, "@")
		if len(parts) == 2 {
			user := parts[0]
			domain := parts[1]
			
			maskedUser := s.maskString(user)
			return maskedUser + "@" + domain
		}
		return s.maskString(match)
	})

	// Remove chaves API completas
	result = apiKeyRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := strings.SplitN(match, "=", 2)
		if len(parts) == 2 {
			return parts[0] + "=***REDACTED***"
		}
		return "***REDACTED***"
	})

	// Remove connection strings
	result = connectionStringRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Mantém protocolo mas remove credenciais
		parts := strings.SplitN(match, "://", 2)
		if len(parts) == 2 {
			hostPart := parts[1]
			if idx := strings.Index(hostPart, "@"); idx != -1 {
				hostPart = hostPart[idx+1:]
			}
			return parts[0] + "://***REDACTED***@" + hostPart
		}
		return "***REDACTED***"
	})

	return result
}

// maskString mascara a maior parte da string, mantendo apenas alguns caracteres
func (s *LogSanitizer) maskString(input string) string {
	if len(input) <= s.showLast {
		return strings.Repeat(s.maskChar, len(input))
	}

	var result strings.Builder
	
	// Adiciona caracteres iniciais se configurado
	if s.showFirst > 0 && len(input) > s.showFirst {
		result.WriteString(input[:s.showFirst])
		result.WriteString(strings.Repeat(s.maskChar, len(input)-s.showFirst-s.showLast))
	} else {
		result.WriteString(strings.Repeat(s.maskChar, len(input)-s.showLast))
	}
	
	// Adiciona últimos caracteres
	if s.showLast > 0 {
		result.WriteString(input[len(input)-s.showLast:])
	}

	return result.String()
}

// SanitizeMap aplica sanitização em todos os valores de um mapa
func (s *LogSanitizer) SanitizeMap(input map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range input {
		switch val := v.(type) {
		case string:
			result[k] = s.Sanitize(val)
		default:
			result[k] = v
		}
	}
	return result
}

// SanitizeCPF mascara especificamente um CPF
func SanitizeCPF(cpf string) string {
	digits := regexp.MustCompile(`\d`).FindAllString(cpf, -1)
	if len(digits) != 11 {
		return cpf
	}
	return "***." + strings.Join(digits[3:6], "") + ".***-" + strings.Join(digits[9:], "")
}

// SanitizeCard mascara número de cartão mostrando apenas últimos 4 dígitos
func SanitizeCard(card string) string {
	digits := regexp.MustCompile(`\d`).FindAllString(card, -1)
	if len(digits) < 4 {
		return strings.Repeat("*", len(digits))
	}
	return "****-****-****-" + strings.Join(digits[len(digits)-4:], "")
}

// SanitizeEmail mascara email mantendo domínio visível
func SanitizeEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***"
	}
	
	user := parts[0]
	domain := parts[1]
	
	if len(user) > 2 {
		user = user[:1] + strings.Repeat("*", len(user)-2) + user[len(user)-1:]
	} else {
		user = "**"
	}
	
	return user + "@" + domain
}
