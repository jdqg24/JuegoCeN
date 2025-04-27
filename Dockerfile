# syntax=docker/dockerfile:1
# Usamos la imagen base de Go para la compilación
FROM golang:1.21 as builder

# Establecemos el directorio de trabajo dentro del contenedor
WORKDIR /app

# Copiamos los archivos de dependencias de Go
COPY go.mod go.sum ./
RUN go mod download

# Copiamos todo el código fuente del proyecto
COPY . .

# Construimos el servidor y cliente
RUN go build -o server ./server
RUN go build -o client ./client

# Usamos una imagen más ligera para ejecutar el contenedor
FROM golang:1.21-alpine

# Copiamos el servidor y cliente desde el contenedor builder
COPY --from=builder /app/server /app/server
COPY --from=builder /app/client /app/client

# Establecemos el directorio de trabajo
WORKDIR /app

# Exponemos el puerto en el que el servidor estará corriendo
EXPOSE 50051

# Comando por defecto para ejecutar el servidor
CMD ["./server"]