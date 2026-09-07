package gateway

import "math"

// ToCents converte um valor em reais (float64) para centavos (int64) com
// arredondamento seguro — evita truncamento (ex: 19.99*100 = 1998.9999999999998
// → int64() trunca para 1997/1998 dependendo do valor, cobrando 1 centavo a
// menos). Usar sempre esta função ao converter reais → centavos em qualquer
// adapter de gateway.
func ToCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}
