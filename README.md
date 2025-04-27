# Proyecto Juego gRPC en Go

## Estructura
- **proto/**: Definición de servicios gRPC
- **server/**: Servidor que maneja movimientos de jugadores
- **client/**: Cliente que envía acciones

## Comandos útiles
```bash
make generate
make run-server
make run-client
```

## Docker
```bash
docker build -t juego-server .
docker run -p 50051:50051 juego-server
```
