package handlers

import "log"

// PublishMessage envia uma mensagem para a fila de pedidos.
// NOTA: RabbitMQ foi removido do sistema. Esta funcao e um stub.
// A fila real e gerenciada pelo monolito via Redis.
func PublishMessage(body []byte) error {
	log.Println("[QUEUE] RabbitMQ removido — mensagem ignorada (fila via Redis no monolito)")
	return nil
}
