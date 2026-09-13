package secretbox

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// chave de teste: 32 bytes em hex. Não é segredo real — é 00..1f.
const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func novoBox(t *testing.T) *Box {
	t.Helper()
	b, err := New(testKey)
	if err != nil {
		t.Fatalf("New com chave válida falhou: %v", err)
	}
	return b
}

func TestRoundTrip(t *testing.T) {
	b := novoBox(t)
	casos := []string{
		"",                       // vazio é válido (token pode ser limpo)
		"APP_USR-1234567890",     // formato de token do MP
		"refresh-abc.def.ghi",    // refresh token
		strings.Repeat("x", 512), // token longo
	}
	for _, original := range casos {
		blob, err := b.SealString(original)
		if err != nil {
			t.Fatalf("Seal(%q): %v", original, err)
		}
		volta, err := b.OpenString(blob)
		if err != nil {
			t.Fatalf("Open do que acabei de Seal(%q): %v", original, err)
		}
		if volta != original {
			t.Errorf("round-trip: obtive %q, queria %q", volta, original)
		}
	}
}

// TestSealNaoVazaOPlaintext: o token não pode aparecer em claro dentro do blob.
// Se aparecesse, cifrar não teria servido para nada.
func TestSealNaoVazaOPlaintext(t *testing.T) {
	b := novoBox(t)
	segredo := "APP_USR-token-do-lojista"
	blob, err := b.SealString(segredo)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(blob, []byte(segredo)) {
		t.Fatal("o plaintext aparece dentro do blob cifrado — a cifragem não protegeu nada")
	}
}

// TestNonceAleatorio: dois Seal do MESMO texto produzem blobs diferentes.
// Sem isso, o banco revelaria quais lojistas têm o mesmo token, e um mesmo
// nonce reusado quebra a segurança do GCM.
func TestNonceAleatorio(t *testing.T) {
	b := novoBox(t)
	a, err := b.SealString("mesmo-token")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	c, err := b.SealString("mesmo-token")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Equal(a, c) {
		t.Fatal("dois Seal do mesmo texto deram o mesmo blob — nonce não é aleatório")
	}
}

// TestOpenDetectaAdulteracao é a garantia central: um byte trocado no banco tem
// que fazer Open FALHAR, não devolver um token corrompido que seria usado para
// tentar cobrar em nome do lojista.
func TestOpenDetectaAdulteracao(t *testing.T) {
	b := novoBox(t)
	blob, err := b.Seal([]byte("APP_USR-token"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// Vira o último byte (dentro do tag de autenticação do GCM).
	adulterado := append([]byte(nil), blob...)
	adulterado[len(adulterado)-1] ^= 0x01
	if _, err := b.Open(adulterado); err != ErrCiphertext {
		t.Errorf("Open de blob adulterado: obtive %v, queria ErrCiphertext", err)
	}

	// Vira um byte no meio do ciphertext.
	meio := append([]byte(nil), blob...)
	meio[len(meio)/2] ^= 0xFF
	if _, err := b.Open(meio); err != ErrCiphertext {
		t.Errorf("Open de ciphertext adulterado no meio: obtive %v, queria ErrCiphertext", err)
	}
}

// TestOpenBlobCurto: blob menor que o nonce não pode causar panic nem devolver
// dado — tem que ser ErrCiphertext.
func TestOpenBlobCurto(t *testing.T) {
	b := novoBox(t)
	for _, curto := range [][]byte{nil, {}, []byte("abc"), make([]byte, 11)} {
		if _, err := b.Open(curto); err != ErrCiphertext {
			t.Errorf("Open(%d bytes): obtive %v, queria ErrCiphertext", len(curto), err)
		}
	}
}

// TestChaveErradaNaoAbre: um blob cifrado com uma chave não abre com outra.
// É o que garante que rotacionar a chave invalida tokens antigos (e que um
// atacante sem a chave não decifra nada).
func TestChaveErradaNaoAbre(t *testing.T) {
	b1 := novoBox(t)
	outraChave := "1f1e1d1c1b1a191817161514131211100f0e0d0c0b0a09080706050403020100"
	b2, err := New(outraChave)
	if err != nil {
		t.Fatalf("New outra chave: %v", err)
	}
	blob, err := b1.SealString("segredo")
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := b2.Open(blob); err != ErrCiphertext {
		t.Errorf("Open com a chave errada: obtive %v, queria ErrCiphertext", err)
	}
}

func TestNewRejeitaChaveRuim(t *testing.T) {
	casos := []struct {
		nome string
		key  string
	}{
		{"vazia", ""},
		{"curta", "0011"},
		{"32 bytes crus mas não-hex", strings.Repeat("z", 64)},
		{"tamanho certo em bytes, errado em hex", strings.Repeat("0", 32)}, // 32 chars = 16 bytes
		{"longa demais", strings.Repeat("0", 128)},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if _, err := New(c.key); err == nil {
				t.Errorf("New(%q) devia falhar mas passou", c.nome)
			}
		})
	}
}

func TestFromEnv(t *testing.T) {
	t.Run("sem a var: falha fechada", func(t *testing.T) {
		t.Setenv("FUU_TEST_KEY", "")
		if _, err := FromEnv("FUU_TEST_KEY"); err == nil {
			t.Error("FromEnv sem a var devia falhar — falha fechada")
		}
	})
	t.Run("com a var: monta", func(t *testing.T) {
		t.Setenv("FUU_TEST_KEY", testKey)
		b, err := FromEnv("FUU_TEST_KEY")
		if err != nil || b == nil {
			t.Fatalf("FromEnv com chave válida: box=%v err=%v", b, err)
		}
	})
}

// sanidade: a chave de teste realmente decodifica para 32 bytes.
func TestChaveDeTesteTem32Bytes(t *testing.T) {
	k, err := hex.DecodeString(testKey)
	if err != nil || len(k) != keyBytes {
		t.Fatalf("chave de teste inválida: len=%d err=%v", len(k), err)
	}
}
