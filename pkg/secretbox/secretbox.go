// Package secretbox cifra segredos curtos (tokens OAuth de gateway) antes de
// gravá-los no banco.
//
// # POR QUE EXISTE
//
// O modelo de marketplace guarda o access_token e o refresh_token da conta
// Mercado Pago de cada estabelecimento. Esses tokens movem o dinheiro da venda
// para a conta do lojista — quem os tem, cobra em nome dele. Em texto puro no
// banco, um dump de tabela ou um backup vazado entrega o controle financeiro
// de todos os lojistas de uma vez. A regra do projeto (CLAUDE.md) já registra
// vazamento recorrente de credencial; token de pagamento é a pior versão disso.
//
// # DESENHO
//
// AES-256-GCM. GCM é autenticado: se o ciphertext for adulterado (um byte
// trocado no banco, um blob truncado), Open FALHA em vez de devolver lixo — o
// que impede usar um token corrompido para tentar cobrar. Cada Seal usa um
// nonce aleatório de 12 bytes, prefixado ao ciphertext; dois Seal do mesmo
// texto produzem blobs diferentes, então o banco não revela quais lojistas
// compartilham algum valor.
//
// A chave vem de SECRET_ENCRYPTION_KEY (env), 32 bytes em hex — gere com
// `openssl rand -hex 32`, o mesmo padrão do METRICS_TOKEN. A chave NUNCA entra
// no repositório nem em doc; vive só no gerenciador de segredos do ambiente.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

// keyBytes é o tamanho da chave AES-256.
const keyBytes = 32

var (
	// ErrKeyFormat indica chave ausente ou fora do formato (64 hex → 32 bytes).
	ErrKeyFormat = errors.New("secretbox: chave precisa ser 64 caracteres hex (32 bytes) — gere com `openssl rand -hex 32`")

	// ErrCiphertext indica blob curto demais ou adulterado. Open devolve isto
	// em vez de dados corrompidos: um token que não abre não pode ser usado
	// para cobrar em nome de ninguém.
	ErrCiphertext = errors.New("secretbox: ciphertext inválido ou adulterado")
)

// Box cifra e decifra com uma chave fixa. Seguro para uso concorrente: o
// cipher.AEAD do stdlib é read-only depois de criado.
type Box struct {
	aead cipher.AEAD
}

// New cria um Box a partir da chave em hex (64 caracteres → 32 bytes).
func New(keyHex string) (*Box, error) {
	if len(keyHex) != hex.EncodedLen(keyBytes) {
		return nil, ErrKeyFormat
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, ErrKeyFormat
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		// aes.NewCipher só falha por tamanho de chave, já validado acima —
		// mas não engolimos: um erro aqui é bug de programação, não de config.
		return nil, fmt.Errorf("secretbox: criar cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secretbox: criar GCM: %w", err)
	}
	return &Box{aead: aead}, nil
}

// FromEnv monta o Box a partir da variável de ambiente nomeada.
//
// Falha fechada: sem a chave, não há cofre, e o serviço não deve subir fingindo
// que guarda token com segurança. Quem chama trata o erro na inicialização.
func FromEnv(envVar string) (*Box, error) {
	v := os.Getenv(envVar)
	if v == "" {
		return nil, fmt.Errorf("%s não configurada: %w", envVar, ErrKeyFormat)
	}
	return New(v)
}

// Seal cifra plaintext e devolve nonce||ciphertext, pronto para gravar em
// coluna BYTEA. Nonce aleatório por chamada.
func (b *Box) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("secretbox: gerar nonce: %w", err)
	}
	// Seal anexa o ciphertext ao dst (o próprio nonce), então o retorno já é
	// nonce||ciphertext num único slice.
	return b.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decifra um blob produzido por Seal. Devolve ErrCiphertext se o blob for
// curto demais ou tiver sido adulterado — nunca dados parciais.
func (b *Box) Open(blob []byte) ([]byte, error) {
	ns := b.aead.NonceSize()
	if len(blob) < ns {
		return nil, ErrCiphertext
	}
	nonce, ct := blob[:ns], blob[ns:]
	plaintext, err := b.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, ErrCiphertext
	}
	return plaintext, nil
}

// SealString e OpenString são conveniências para o caso comum (o token é uma
// string).
func (b *Box) SealString(s string) ([]byte, error) { return b.Seal([]byte(s)) }

func (b *Box) OpenString(blob []byte) (string, error) {
	p, err := b.Open(blob)
	if err != nil {
		return "", err
	}
	return string(p), nil
}
