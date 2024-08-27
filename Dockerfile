# Etapa de construção
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY cmd/restclient/ ./cmd/restclient/

RUN CGO_ENABLED=0 GOOS=linux go build -o restclient ./cmd/restclient/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/restclient .

ENTRYPOINT ["./restclient"]
