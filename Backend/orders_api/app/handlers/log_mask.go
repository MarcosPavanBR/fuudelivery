package handlers

import "strings"

// maskPhone reduz o telefone do cliente ao que basta para correlacionar um
// log (os 4 últimos dígitos). Telefone é dado pessoal (LGPD) e log não tem
// controle de acesso nem retenção definidos.
func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) <= 4 {
		return "***"
	}
	return "***" + phone[len(phone)-4:]
}
