package models

import (
	"regexp"
	"strings"
)

// DeliveryRegionFee é uma faixa de CEP com preço de frete fixo.
//
// Por que por REGIÃO e não por km: o app calculava a distância a partir do GPS
// do celular no momento do pedido, não do endereço de entrega — quem pedia do
// trabalho para casa era cobrado pela distância até o trabalho. E a distância
// vinha no CORPO da requisição, então mandar `"distance": 0` pagava só a taxa
// fixa. Preço por faixa de CEP resolve os dois: o número que decide o preço é
// o endereço para onde a comida vai, e não exige geocodificação (o CEP já
// chega preenchido pelo ViaCEP no app).
type DeliveryRegionFee struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:100;not null" json:"name"`

	// Faixa de CEP, 8 dígitos, inclusiva nas duas pontas. Guardada como
	// inteiro para a comparação ser numérica: como texto, "01000000" <
	// "9" seria verdadeiro e a faixa casaria errado.
	CepStart int `gorm:"not null;index" json:"cep_start"`
	CepEnd   int `gorm:"not null;index" json:"cep_end"`

	// Opcionais, para desempatar CEP igual em cidade diferente. Vazio = não
	// restringe.
	City string `gorm:"size:100" json:"city"`
	UF   string `gorm:"size:2;column:uf" json:"uf"`

	Fee float64 `gorm:"not null" json:"fee"`

	// Menor ganha. Serve para uma regra estreita ("Centro histórico") vencer
	// uma ampla ("Centro") sem depender da ordem de inserção.
	Priority int `gorm:"not null;default:100" json:"priority"`

	// SEM `default:true` na tag, de propósito. Com o default declarado, o GORM
	// OMITE o campo no INSERT quando o valor é o zero do tipo — ou seja,
	// gravar Active=false escrevia o default TRUE e a regra "desativada"
	// continuava precificando pedido. Peguei isso num teste que eu mesmo
	// escrevi esperando o contrário.
	//
	// O DEFAULT continua existindo no banco (sql/24), para INSERT em SQL cru
	// que não cite a coluna. Aqui o Go sempre manda o valor explícito.
	Active bool `gorm:"not null" json:"active"`
}

func (DeliveryRegionFee) TableName() string { return "delivery_region_fees" }

var apenasDigitos = regexp.MustCompile(`\D`)

// NormalizeCep devolve o CEP como inteiro de 8 dígitos, ou 0 se não der.
//
// Aceita "01310-100", "01310100" e " 01310100 " — o app manda só dígitos, mas
// o admin digita com traço e um pedido antigo pode ter qualquer coisa.
func NormalizeCep(cep string) int {
	d := apenasDigitos.ReplaceAllString(strings.TrimSpace(cep), "")
	if len(d) != 8 {
		return 0
	}
	n := 0
	for _, r := range d {
		n = n*10 + int(r-'0')
	}
	return n
}

// ResolveRegionFee acha a regra que vale para um endereço.
//
// Critério de desempate, nesta ordem: menor `priority`, depois faixa mais
// ESTREITA. A largura importa porque é o que faz "Centro histórico"
// (01000000-01009999) vencer "São Paulo capital" (01000000-05999999) sem que
// alguém precise lembrar de ajustar prioridade — o mais específico ganha
// sozinho.
//
// Devolve false quando nada casa; o chamador decide o que fazer, e a decisão
// NUNCA é frete grátis.
func ResolveRegionFee(cep, city, uf string) (DeliveryRegionFee, bool) {
	n := NormalizeCep(cep)
	if n == 0 || DB == nil {
		return DeliveryRegionFee{}, false
	}

	var regras []DeliveryRegionFee
	q := DB.Where("active = ? AND cep_start <= ? AND cep_end >= ?", true, n, n)

	// City/UF só filtram quando a REGRA os declara. Regra sem cidade vale para
	// qualquer cidade; regra com cidade só vale para a dela. Comparação
	// case-insensitive porque "São Paulo" e "SAO PAULO" chegam dos dois.
	q = q.Where("(city = '' OR LOWER(city) = LOWER(?))", strings.TrimSpace(city))
	q = q.Where("(uf = '' OR LOWER(uf) = LOWER(?))", strings.TrimSpace(uf))

	if err := q.Order("priority ASC, (cep_end - cep_start) ASC, id ASC").
		Limit(1).Find(&regras).Error; err != nil || len(regras) == 0 {
		return DeliveryRegionFee{}, false
	}
	return regras[0], true
}
