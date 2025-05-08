package main

import (
	"io"
	"log"
	"sync"
	"time"

	"net"

	pb "JuegoCeN/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedPingPongServer
	// estado compartido
	mu      sync.Mutex
	state   *pb.GameState
	clients map[pb.PingPong_PlayServer]struct{}
}

func newServer() *server {
	return &server{
		state: &pb.GameState{
			Ball:    &pb.Position{X: 0.5, Y: 0.5},
			Paddle1: &pb.Position{X: 0.05, Y: 0.5},
			Paddle2: &pb.Position{X: 0.95, Y: 0.5},
			Score1:  0,
			Score2:  0,
		},
		clients: make(map[pb.PingPong_PlayServer]struct{}),
	}
}

func (s *server) Play(stream pb.PingPong_PlayServer) error {
	// registrar cliente
	s.mu.Lock()
	s.clients[stream] = struct{}{}
	s.mu.Unlock()

	// canal para recibir acciones desde este cliente
	go func() {
		for {
			action, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("Recv error: %v", err)
				break
			}
			s.applyAction(action)
		}
	}()

	// enviar estados periódicamente
	ticker := time.NewTicker(16 * time.Millisecond) // ~60 FPS
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		state := s.state
		s.mu.Unlock()

		if err := stream.Send(state); err != nil {
			log.Printf("Send error: %v", err)
			break
		}
	}

	// cleanup al desconectar
	s.mu.Lock()
	delete(s.clients, stream)
	s.mu.Unlock()
	return nil
}

func (s *server) applyAction(a *pb.GameAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Mover paleta según a.PlayerId y a.Move (“UP”/“DOWN”)
	var paddle *pb.Position
	if a.PlayerId == "1" {
		paddle = s.state.Paddle1
	} else {
		paddle = s.state.Paddle2
	}
	const delta = 0.02
	if a.Move == "UP" && paddle.Y+delta <= 1.0 {
		paddle.Y += delta
	}
	if a.Move == "DOWN" && paddle.Y-delta >= 0.0 {
		paddle.Y -= delta
	}
	// lógica de bola (choques, puntuación) omitida por brevedad…
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterPingPongServer(grpcServer, newServer())
	reflection.Register(grpcServer)
	log.Println("Server gRPC corriendo en :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Serve failed: %v", err)
	}
}
