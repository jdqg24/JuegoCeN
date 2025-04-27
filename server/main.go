package main

import (
	"context"
	"log"
	"net"

	pb "JuegoCeN/proto"

	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedJuegoServiceServer
}

func (s *server) MoverJugador(ctx context.Context, req *pb.MovimientoRequest) (*pb.MovimientoResponse, error) {
	log.Printf("Jugador %s se movió hacia %s", req.JugadorId, req.Direccion)
	return &pb.MovimientoResponse{Resultado: "Movimiento exitoso"}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("falló al escuchar: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterJuegoServiceServer(s, &server{})
	log.Println("Servidor escuchando en :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("falló al servir: %v", err)
	}
}
