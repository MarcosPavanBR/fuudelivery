FROM golang:1.25-alpine AS builder
# Set the working directory inside the container
WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/app

FROM scratch
WORKDIR /usr/src/app
COPY --from=builder /usr/local/bin/app .
CMD [ "./app" ]
