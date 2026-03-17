FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /shipment-server ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache curl \
    && curl -fsSL https://github.com/golang-migrate/migrate/releases/download/v4.18.2/migrate.linux-amd64.tar.gz \
       | tar -xz -C /usr/local/bin \
    && chmod +x /usr/local/bin/migrate \
    && apk del curl

COPY --from=builder /shipment-server /shipment-server
COPY --from=builder /app/migrations /migrations
COPY --from=builder /app/config /config

EXPOSE 50051

ENTRYPOINT ["/shipment-server"]
