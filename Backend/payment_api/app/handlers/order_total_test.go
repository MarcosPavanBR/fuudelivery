package handlers

import (
	"errors"
	"testing"
)

// canViewOrderPayment é a decisão de autorização de GetPaymentByOrder
// (auditoria de segurança — achado #3: consulta de cobrança de pedido alheio).
func TestCanViewOrderPayment_Admin(t *testing.T) {
	if !canViewOrderPayment("admin", "", errors.New("sem phone"), 0, errors.New("sem est"), "+5511999900001", 1) {
		t.Error("admin deveria consultar qualquer cobrança")
	}
}

func TestCanViewOrderPayment_CustomerOwner(t *testing.T) {
	if !canViewOrderPayment("client", "+5511999900001", nil, 0, errors.New("sem est"), "+5511999900001", 1) {
		t.Error("cliente dono do pedido deveria ver a própria cobrança")
	}
}

func TestCanViewOrderPayment_EstablishmentOwner(t *testing.T) {
	if !canViewOrderPayment("restaurant", "", errors.New("sem phone"), 7, nil, "+5511999900001", 7) {
		t.Error("estabelecimento dono da cobrança deveria ver")
	}
}

func TestCanViewOrderPayment_AnotherCustomerDenied(t *testing.T) {
	if canViewOrderPayment("client", "+5511999988888", nil, 0, errors.New("sem est"), "+5511999900001", 1) {
		t.Error("outro cliente NÃO deveria ver cobrança alheia (IDOR)")
	}
}

func TestCanViewOrderPayment_AnotherEstablishmentDenied(t *testing.T) {
	if canViewOrderPayment("restaurant", "", errors.New("sem phone"), 8, nil, "+5511999900001", 7) {
		t.Error("outro estabelecimento NÃO deveria ver cobrança de pedido de outro tenant")
	}
}

func TestCanViewOrderPayment_DeliverymanDenied(t *testing.T) {
	// Entregador não é participante autorizado na implementação atual: o phone
	// do token não é o do cliente e ele não tem establishment_id do pedido —
	// papel sozinho nunca concede acesso (sem vazamento).
	if canViewOrderPayment("deliverer", "+5511999888888", nil, 0, errors.New("sem est"), "+5511999900001", 1) {
		t.Error("entregador não deveria consultar cobrança de pedido que não é dele")
	}
}

func TestCanViewOrderPayment_NoClaimsDenied(t *testing.T) {
	if canViewOrderPayment("client", "", errors.New("sem phone"), 0, errors.New("sem est"), "+5511999900001", 1) {
		t.Error("usuário sem claims não deveria passar")
	}
}
