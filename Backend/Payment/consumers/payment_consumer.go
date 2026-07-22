// Package consumers gerencia consumidores de mensagens RabbitMQ.
// O PaymentConsumer escuta a fila de pagamentos e processa
// aprovacoes automaticamente, creditando valores nas carteiras.
package consumers

import (
	"encoding/json"
	"log"

	"github.com/streadway/amqp"
	"github.com/carloshomar/vercardapio/payment/config"
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/services"
)

// PaymentConsumer escuta a fila RabbitMQ e processa pagamentos.
type PaymentConsumer struct {
	Conn    *amqp.Connection        // Conexao RabbitMQ
	Channel *amqp.Channel           // Canal de comunicacao
	Wallet  *services.WalletService // Servico de carteiras
}

// NewPaymentConsumer cria uma nova instancia do consumer.
// Estabelece conexao com o RabbitMQ.
func NewPaymentConsumer() (*PaymentConsumer, error) {
	cfg := config.AppConfig

	// Conecta ao RabbitMQ
	conn, err := amqp.Dial(cfg.RabbitConnection)
	if err != nil {
		return nil, err
	}

	// Abre um canal de comunicacao
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	return &PaymentConsumer{
		Conn:    conn,
		Channel: ch,
		Wallet:  services.NewWalletService(),
	}, nil
}

// Start inicia o consumer: declara a fila e comeca a consumir mensagens.
// Cada mensagem e processada em uma goroutine separada.
func (pc *PaymentConsumer) Start() error {
	cfg := config.AppConfig

	// Declara a fila (durable=true para sobreviver a reinicios)
	q, err := pc.Channel.QueueDeclare(
		cfg.RabbitPaymentQueue, // Nome da fila
		true,                   // Durable: sobrevive a restarts
		false,                  // Auto-delete: nao deleta quando vazio
		false,                  // Exclusive: nao e exclusiva
		false,                  // No-wait: nao espera confirmacao
		nil,                    // Args: sem argumentos extras
	)
	if err != nil {
		return err
	}

	// Consome mensagens da fila
	// auto-ack=true: confirma recebimento imediatamente
	msgs, err := pc.Channel.Consume(
		q.Name,  // Fila
		"",      // Consumer tag (vazio = auto)
		true,    // Auto-ack
		false,   // Exclusive
		false,   // No-local
		false,   // No-wait
		nil,     // Args
	)
	if err != nil {
		return err
	}

	// Inicia goroutine para processar mensagens
	go func() {
		for msg := range msgs {
			pc.processMessage(msg)
		}
	}()

	log.Println("Payment consumer started")
	return nil
}

// processMessage processa uma mensagem recebida da fila.
// Se o pagamento estiver aprovado, credita o valor na carteira do restaurante.
func (pc *PaymentConsumer) processMessage(msg amqp.Delivery) {
	// Deserializa o pagamento
	var payment models.Payment
	if err := json.Unmarshal(msg.Body, &payment); err != nil {
		log.Printf("Error unmarshaling payment: %v", err)
		return
	}

	log.Printf("Processing payment: %s", payment.OrderID)

	// Se aprovado, credita na carteira
	if payment.Status == models.PaymentApproved {
		if err := pc.Wallet.ProcessPaymentApproval(&payment); err != nil {
			log.Printf("Error processing payment approval: %v", err)
		}
	}
}

// Stop encerra a conexao com o RabbitMQ.
func (pc *PaymentConsumer) Stop() {
	if pc.Channel != nil {
		pc.Channel.Close()
	}
	if pc.Conn != nil {
		pc.Conn.Close()
	}
}
