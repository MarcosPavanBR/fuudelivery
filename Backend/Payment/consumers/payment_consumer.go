package consumers

import (
	"encoding/json"
	"log"
	"time"

	"github.com/streadway/amqp"
	"github.com/carloshomar/vercardapio/payment/config"
	"github.com/carloshomar/vercardapio/payment/models"
	"github.com/carloshomar/vercardapio/payment/services"
)

type PaymentConsumer struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
	Wallet  *services.WalletService
}

func NewPaymentConsumer() (*PaymentConsumer, error) {
	cfg := config.AppConfig

	conn, err := amqp.Dial(cfg.RabbitConnection)
	if err != nil {
		return nil, err
	}

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

func (pc *PaymentConsumer) Start() error {
	cfg := config.AppConfig

	q, err := pc.Channel.QueueDeclare(
		cfg.RabbitPaymentQueue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	msgs, err := pc.Channel.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for msg := range msgs {
			pc.processMessage(msg)
		}
	}()

	log.Println("Payment consumer started")
	return nil
}

func (pc *PaymentConsumer) processMessage(msg amqp.Delivery) {
	var payment models.Payment
	if err := json.Unmarshal(msg.Body, &payment); err != nil {
		log.Printf("Error unmarshaling payment: %v", err)
		return
	}

	log.Printf("Processing payment: %s", payment.OrderID)

	if payment.Status == models.PaymentApproved {
		if err := pc.Wallet.ProcessPaymentApproval(&payment); err != nil {
			log.Printf("Error processing payment approval: %v", err)
		}
	}
}

func (pc *PaymentConsumer) Stop() {
	if pc.Channel != nil {
		pc.Channel.Close()
	}
	if pc.Conn != nil {
		pc.Conn.Close()
	}
}

func init() {
	_ = time.Now()
}
