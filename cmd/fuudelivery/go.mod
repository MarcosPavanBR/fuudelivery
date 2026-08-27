module github.com/carloshomar/fuudelivery

go 1.25.0

require (
	github.com/alicebob/miniredis/v2 v2.38.0
	github.com/carloshomar/fuudelivery/auth_api v0.0.0
	github.com/carloshomar/fuudelivery/chat_api v0.0.0-00010101000000-000000000000
	github.com/carloshomar/fuudelivery/delivery_api v0.0.0-00010101000000-000000000000
	github.com/carloshomar/fuudelivery/orders_api v0.0.0-00010101000000-000000000000
	github.com/carloshomar/fuudelivery/payment_api v0.0.0-00010101000000-000000000000
	github.com/carloshomar/fuudelivery/pkg/health v0.0.0
	github.com/carloshomar/fuudelivery/pkg/queue v0.0.0
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gofiber/contrib/websocket v1.3.4
	github.com/gofiber/fiber/v2 v2.52.15
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/joho/godotenv v1.5.1
	github.com/stretchr/testify v1.12.1
	golang.org/x/crypto v0.55.0
	golang.org/x/time v0.14.0
	gorm.io/driver/postgres v1.6.2
	gorm.io/gorm v1.31.2
)

require (
	dario.cat/mergo v1.0.2 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/ebitengine/purego v0.10.1 // indirect
	github.com/fasthttp/websocket v1.5.12 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/savsgio/gotils v0.0.0-20250408102913-196191ec6287 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.61.0 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.70.0 // indirect
	go.opentelemetry.io/otel v1.45.0 // indirect
	go.opentelemetry.io/otel/metric v1.45.0 // indirect
	go.opentelemetry.io/otel/trace v1.45.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

replace github.com/carloshomar/fuudelivery/auth_api => ../../Backend/auth_api

replace github.com/carloshomar/fuudelivery/orders_api => ../../Backend/orders_api

replace github.com/carloshomar/fuudelivery/delivery_api => ../../Backend/delivery_api

replace github.com/carloshomar/fuudelivery/payment_api => ../../Backend/payment_api

replace github.com/carloshomar/fuudelivery/chat_api => ../../Backend/chat_api

replace github.com/carloshomar/fuudelivery/pkg/queue => ../../pkg/queue

replace github.com/carloshomar/fuudelivery/pkg/health => ../../pkg/health
