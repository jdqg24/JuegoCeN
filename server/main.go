package main

import (
	"flag"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"

	pb "JuegoCeN/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// server implements the PingPong gRPC service
type server struct {
	pb.UnimplementedPingPongServer
	mu       sync.Mutex
	state    *pb.GameState
	clients  map[pb.PingPong_PlayServer]struct{}
	ballVelX float32
	ballVelY float32
}

// newServer initializes the game state and velocities
func newServer() *server {
	return &server{
		state: &pb.GameState{
			Ball:    &pb.Position{X: 0.5, Y: 0.5},
			Paddle1: &pb.Position{X: 0.05, Y: 0.5},
			Paddle2: &pb.Position{X: 0.95, Y: 0.5},
			Score1:  0,
			Score2:  0,
		},
		clients:  make(map[pb.PingPong_PlayServer]struct{}),
		ballVelX: 0.01,
		ballVelY: 0.005,
	}
}

// Play handles bidirectional streaming of game actions and state
func (s *server) Play(stream pb.PingPong_PlayServer) error {
	// Register client
	s.mu.Lock()
	s.clients[stream] = struct{}{}
	s.mu.Unlock()

	// Receive actions from client
	go func() {
		for {
			action, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Printf("Recv error: %v", err)
				return
			}
			s.applyAction(action)
		}
	}()

	// Send updated state at ~60 FPS
	ticker := time.NewTicker(time.Millisecond * 16)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		s.tick()
		st := s.state
		s.mu.Unlock()

		if err := stream.Send(st); err != nil {
			log.Printf("Send error: %v", err)
			break
		}
	}

	// Cleanup on disconnect
	s.mu.Lock()
	delete(s.clients, stream)
	s.mu.Unlock()
	return nil
}

// applyAction moves the paddle according to client input
func (s *server) applyAction(a *pb.GameAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
}

// tick updates ball position, checks collisions, and handles scoring
func (s *server) tick() {
	bs := s.state.Ball
	// Move ball
	bs.X += s.ballVelX
	bs.Y += s.ballVelY

	// Bounce off top/bottom
	if bs.Y <= 0.0 || bs.Y >= 1.0 {
		s.ballVelY *= -1
	}

	// Bounce off paddles
	if bs.X <= s.state.Paddle1.X+0.02 && math.Abs(float64(bs.Y-s.state.Paddle1.Y)) < 0.1 {
		s.ballVelX *= -1
	}
	if bs.X >= s.state.Paddle2.X-0.02 && math.Abs(float64(bs.Y-s.state.Paddle2.Y)) < 0.1 {
		s.ballVelX *= -1
	}

	// Score and reset if out of bounds
	if bs.X < 0.0 {
		s.state.Score2++
		s.resetBall()
	}
	if bs.X > 1.0 {
		s.state.Score1++
		s.resetBall()
	}
}

// resetBall re-centers the ball and reverses X velocity
func (s *server) resetBall() {

	s.state.Ball.X = 0.5
	s.state.Ball.Y = 0.5
	s.ballVelX = -s.ballVelX
	// randomize Y velocity between -0.015 and +0.015
	s.ballVelY = rand.Float32()*0.03 - 0.015
}

func main() {
	// allow port override via flag
	port := flag.String("port", "50051", "The server port")
	flag.Parse()

	addr := ":" + *port
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterPingPongServer(grpcServer, newServer())
	reflection.Register(grpcServer)
	log.Printf("Server gRPC corriendo en %s", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Serve failed: %v", err)
	}
}
