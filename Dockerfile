# 1. Etapa de compilación
FROM golang:1.21 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
WORKDIR /app/server
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# 2. Etapa de ejecución mínima
FROM alpine:latest
RUN apk add --no-cache ca-certificates

WORKDIR /root/
COPY --from=builder /app/server/server .
EXPOSE 50051

ENTRYPOINT ["./server"]
