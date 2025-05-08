package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"time"

	pb "JuegoCeN/proto"

	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewPingPongClient(conn)
	stream, err := client.Play(context.Background())
	if err != nil {
		log.Fatalf("Play failed: %v", err)
	}

	// Goroutine para recibir estados
	go func() {
		for {
			state, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Fatalf("Recv error: %v", err)
			}
			// Aquí podrías renderizar en consola o UI
			log.Printf("Ball at (%.2f, %.2f) | P1: %.2f, P2: %.2f | Score %d:%d",
				state.Ball.X, state.Ball.Y,
				state.Paddle1.Y, state.Paddle2.Y,
				state.Score1, state.Score2,
			)
		}
	}()

	// Leer teclas desde stdin y enviar acciones
	reader := bufio.NewReader(os.Stdin)
	playerID := "1" // o “2” si eres el segundo jugador
	for {
		input, _ := reader.ReadString('\n')
		move := "NONE"
		switch input {
		case "w\n":
			move = "UP"
		case "s\n":
			move = "DOWN"
		}
		action := &pb.GameAction{PlayerId: playerID, Move: move}
		if err := stream.Send(action); err != nil {
			log.Fatalf("Send error: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
