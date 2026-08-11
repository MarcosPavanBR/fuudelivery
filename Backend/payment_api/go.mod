module github.com/carloshomar/vercardapio/payment_api

go 1.25.0

require (
	github.com/carloshomar/vercardapio/auth_api v0.0.0
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/gofiber/contrib/websocket v1.3.4
	github.com/gofiber/fiber/v2 v2.52.12
	github.com/joho/godotenv v1.5.1
	go.mongodb.org/mongo-driver v1.17.9
)

replace github.com/carloshomar/vercardapio/auth_api => ../auth_api

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fasthttp/websocket v1.5.12 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.7.4 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
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
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	gorm.io/driver/postgres v1.5.11 // indirect
	gorm.io/gorm v1.31.2 // indirect
)

replace github.com/carloshomar/fuudelivery/pkg/queue => ../../pkg/queue

require github.com/carloshomar/fuudelivery/pkg/queue v0.0.0

require github.com/carloshomar/fuudelivery/pkg/health v0.0.0

replace github.com/carloshomar/fuudelivery/pkg/health => ../../pkg/health
