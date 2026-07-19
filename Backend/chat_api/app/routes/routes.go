package routes

import (
	"github.com/carloshomar/vercardapio/chat_api/app/handlers"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws/chat/:orderId/:userId/:userType", websocket.New(handlers.HandleChatWebSocket))

	app.Get("/chat/messages/:orderId", handlers.GetMessages)
	app.Post("/chat/message", handlers.SendMessage)
	app.Put("/chat/read/:orderId/:userId", handlers.MarkAsRead)
}
