package main

import (
	"context"
	"log"
	"time"

	pb "JuegoCeN/proto"

	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("no se pudo conectar: %v", err)
	}
	defer conn.Close()
	c := pb.NewJuegoServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.MoverJugador(ctx, &pb.MovimientoRequest{JugadorId: "jugador123", Direccion: "norte"})
	if err != nil {
		log.Fatalf("error al mover jugador: %v", err)
	}
	log.Printf("Respuesta del servidor: %s", r.Resultado)
}
