package handlers

import (
	"errors"
	"testing"
)

// canManageEstablishment é a decisão de autorização compartilhada pelos
// handlers GetUserByEstablishment e HandlerEstablishmentStatus (auditoria de
// segurança — achado #2: enumeração de usuários e toggle de status alheio).
// Regressão real da lógica: admin libera qualquer estabelecimento; demais
// papéis só o próprio.
func TestCanManageEstablishment_AdminCanManageAny(t *testing.T) {
	if !canManageEstablishment("admin", 0, errors.New("sem claim de est"), 999) {
		t.Error("admin deveria gerenciar qualquer estabelecimento")
	}
}

func TestCanManageEstablishment_OwnerOfSameEstablishment(t *testing.T) {
	if !canManageEstablishment("restaurant", 42, nil, 42) {
		t.Error("dono do estabelecimento 42 deveria poder gerenciar o 42")
	}
}

func TestCanManageEstablishment_OtherEstablishmentDenied(t *testing.T) {
	if canManageEstablishment("restaurant", 42, nil, 43) {
		t.Error("usuário do estabelecimento 42 NÃO deveria gerenciar o 43 (IDOR)")
	}
}

func TestCanManageEstablishment_NoEstablishmentLinkDenied(t *testing.T) {
	if canManageEstablishment("client", 0, errors.New("establishment_id ausente no token"), 42) {
		t.Error("usuário sem vínculo a estabelecimento não deveria passar")
	}
}

func TestCanManageEstablishment_InvalidTokenErrDenied(t *testing.T) {
	// eErr não-nulo (token inválido/ausente de claim) bloqueia mesmo com IDs iguais.
	if canManageEstablishment("restaurant", 42, errors.New("token inválido"), 42) {
		t.Error("erro de token deveria bloquear o acesso")
	}
}
