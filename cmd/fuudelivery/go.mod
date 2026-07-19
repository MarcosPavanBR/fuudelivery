module github.com/carloshomar/fuudelivery

go 1.23.0

require (
	github.com/carloshomar/vercardapio/auth_api v0.0.0-00010101000000-000000000000
	github.com/carloshomar/vercardapio/chat_api v0.0.0-00010101000000-000000000000
	github.com/carloshomar/vercardapio/delivery_api v0.0.0-00010101000000-000000000000
	github.com/carloshomar/vercardapio/orders_api v0.0.0-00010101000000-000000000000
	github.com/carloshomar/vercardapio/payment_api v0.0.0-00010101000000-000000000000
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gofiber/contrib/websocket v1.3.4
	github.com/gofiber/fiber/v2 v2.52.6
	github.com/joho/godotenv v1.5.1
	go.mongodb.org/mongo-driver v1.17.3
	gorm.io/gorm v1.25.12
)

require (
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/streadway/amqp v1.1.0 // indirect
	gorm.io/driver/postgres v1.5.11 // indirect
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fasthttp/websocket v1.5.12 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.7.4 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/savsgio/gotils v0.0.0-20250408102913-196191ec6287 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.61.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
)

replace github.com/carloshomar/vercardapio/auth_api => ../../Backend/auth_api

replace github.com/carloshomar/vercardapio/orders_api => ../../Backend/orders_api

replace github.com/carloshomar/vercardapio/delivery_api => ../../Backend/delivery_api

replace github.com/carloshomar/vercardapio/payment_api => ../../Backend/payment_api

replace github.com/carloshomar/vercardapio/chat_api => ../../Backend/chat_api
