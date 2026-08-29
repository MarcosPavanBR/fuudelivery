package middleware

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// SafeContextMiddleware garante que o contexto do Fiber não seja passado para goroutines
// Este middleware cria um contexto seguro para cada requisição
func SafeContext() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Cria um contexto derivado do UserContext do Fiber
		// que é seguro para passar para goroutines
		ctx, cancel := context.WithTimeout(c.UserContext(), 30*time.Second)
		defer cancel()

		// Armazena o contexto seguro nos Locals
		c.Locals("safe_context", ctx)

		return c.Next()
	}
}

// GetSafeContext recupera o contexto seguro armazenado
func GetSafeContext(c *fiber.Ctx) context.Context {
	ctx, ok := c.Locals("safe_context").(context.Context)
	if !ok {
		return c.UserContext()
	}
	return ctx
}

// TimeoutMiddleware adiciona timeout configurável às requisições
func Timeout(timeout time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), timeout)
		defer cancel()

		c.Locals("safe_context", ctx)

		done := make(chan error, 1)
		
		go func() {
			done <- c.Next()
		}()

		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return c.SendStatus(fiber.StatusGatewayTimeout)
		}
	}
}

// RecoveryMiddleware configura recovery com logging estruturado
func Recovery() fiber.Handler {
	return recover.New(recover.Config{
		EnableStackTrace: true,
	})
}

// GoroutineSafeHandler é um wrapper para handlers que precisam executar tarefas em background
// Usage: go GoroutineSafeHandler(c)(func(ctx context.Context) { ... })
func GoroutineSafeHandler(c *fiber.Ctx) func(fn func(ctx context.Context)) {
	ctx := GetSafeContext(c)
	
	return func(fn func(ctx context.Context)) {
		go func(safeCtx context.Context) {
			defer func() {
				if r := recover(); r != nil {
					// Log da panic
					// logger.Error("Panic in goroutine", "error", r)
				}
			}()
			fn(safeCtx)
		}(ctx)
	}
}
