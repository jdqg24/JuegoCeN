// server/main.go
package main

import (
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"

	pb "JuegoCeN/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type GameRoom struct {
	mu       sync.Mutex
	players  []pb.PingPong_PlayServer
	state    *pb.GameState
	velX     float32
	velY     float32
	roomCode string
}

var (
	// Cola de emparejamiento
	waitingQueue   []pb.PingPong_PlayServer
	waitingQueueMu sync.Mutex

	// Mapa para recuperar la sala de un stream
	streamToRoom   = make(map[pb.PingPong_PlayServer]*GameRoom)
	streamToRoomMu sync.Mutex
)

// run envía el estado a ambos jugadores ~60 veces por segundo.
func (gr *GameRoom) run() {
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	const padWidth = 10.0 / 800.0
	const padRange = 0.10

	for range ticker.C {
		gr.mu.Lock()
		if len(gr.players) < 2 {
			gr.mu.Unlock()
			return
		}
		// Física de la bola
		gr.state.Ball.X += gr.velX
		gr.state.Ball.Y += gr.velY
		if gr.state.Ball.Y <= 0 || gr.state.Ball.Y >= 1 {
			gr.velY = -gr.velY
		}
		leftEdge := gr.state.Paddle1.X + padWidth
		if gr.velX < 0 &&
			gr.state.Ball.X <= leftEdge &&
			math.Abs(float64(gr.state.Ball.Y-gr.state.Paddle1.Y)) < padRange {
			gr.state.Ball.X = leftEdge
			gr.velX = -gr.velX
		}
		rightEdge := gr.state.Paddle2.X - padWidth
		if gr.velX > 0 &&
			gr.state.Ball.X >= rightEdge &&
			math.Abs(float64(gr.state.Ball.Y-gr.state.Paddle2.Y)) < padRange {
			gr.state.Ball.X = rightEdge
			gr.velX = -gr.velX
		}
		if gr.state.Ball.X < 0 {
			gr.state.Score2++
			gr.state.Ball.X, gr.state.Ball.Y = 0.5, 0.5
		} else if gr.state.Ball.X > 1 {
			gr.state.Score1++
			gr.state.Ball.X, gr.state.Ball.Y = 0.5, 0.5
		}

		// Clonar datos para envío
		st := gr.state
		pls := append([]pb.PingPong_PlayServer(nil), gr.players...)
		gr.mu.Unlock()

		// Enviar a cada jugador con su ID
		for i, p := range pls {
			msg := &pb.GameState{
				RoomCode: st.RoomCode,
				Ball:     &pb.Vector{X: st.Ball.X, Y: st.Ball.Y},
				Paddle1:  &pb.Vector{X: st.Paddle1.X, Y: st.Paddle1.Y},
				Paddle2:  &pb.Vector{X: st.Paddle2.X, Y: st.Paddle2.Y},
				Score1:   st.Score1,
				Score2:   st.Score2,
				PlayerId: fmt.Sprintf("%d", i+1),
			}
			if err := p.Send(msg); err != nil {
				log.Printf("Error enviando estado al jugador %d: %v", i+1, err)
			}
		}
	}
}

// handleAction mueve las paletas según la acción recibida.
func (gr *GameRoom) handleAction(a *pb.GameAction) {
	gr.mu.Lock()
	defer gr.mu.Unlock()

	if gr.state == nil {
		return
	}
	const delta = 0.02

	switch a.PlayerId {
	case "1":
		switch a.Move {
		case "UP":
			gr.state.Paddle1.Y -= delta
		case "DOWN":
			gr.state.Paddle1.Y += delta
			// case "NONE": no hacemos nada
		}
	case "2":
		switch a.Move {
		case "UP":
			gr.state.Paddle2.Y -= delta
		case "DOWN":
			gr.state.Paddle2.Y += delta
			// case "NONE": no hacemos nada
		}
	}

	// Limitar dentro de [0,1]
	if gr.state.Paddle1.Y < 0 {
		gr.state.Paddle1.Y = 0
	} else if gr.state.Paddle1.Y > 1 {
		gr.state.Paddle1.Y = 1
	}
	if gr.state.Paddle2.Y < 0 {
		gr.state.Paddle2.Y = 0
	} else if gr.state.Paddle2.Y > 1 {
		gr.state.Paddle2.Y = 1
	}
}

type server struct{ pb.UnimplementedPingPongServer }

// Play implementa emparejamiento automático por parejas.
func (s *server) Play(stream pb.PingPong_PlayServer) error {
	// 1) Primer recv para disparar emparejamiento
	if _, err := stream.Recv(); err != nil {
		return err
	}

	var room *GameRoom

	// 2) Emparejamiento en parejas
	waitingQueueMu.Lock()
	if len(waitingQueue) == 0 {
		// Primer jugador se queda en cola
		waitingQueue = append(waitingQueue, stream)
		waitingQueueMu.Unlock()
		// Espera hasta que sea emparejado
		for {
			time.Sleep(50 * time.Millisecond)
			waitingQueueMu.Lock()
			found := false
			for _, p := range waitingQueue {
				if p == stream {
					found = true
					break
				}
			}
			waitingQueueMu.Unlock()
			if !found {
				break
			}
		}
	} else {
		// Segundo jugador empareja con el primero
		peer := waitingQueue[0]
		waitingQueue = waitingQueue[1:]
		waitingQueueMu.Unlock()

		// Crear sala nueva
		room = &GameRoom{
			velX:     0.008,
			velY:     0.012,
			roomCode: fmt.Sprintf("%04X", time.Now().UnixNano()%0x10000),
		}
		// Inicializar estado
		room.state = &pb.GameState{
			RoomCode: room.roomCode,
			Ball:     &pb.Vector{X: 0.5, Y: 0.5},
			Paddle1:  &pb.Vector{X: 0.1, Y: 0.5},
			Paddle2:  &pb.Vector{X: 0.9, Y: 0.5},
			Score1:   0,
			Score2:   0,
		}
		// Añadir ambos
		room.players = []pb.PingPong_PlayServer{peer, stream}

		// Mapear streams a sala
		streamToRoomMu.Lock()
		streamToRoom[peer] = room
		streamToRoom[stream] = room
		streamToRoomMu.Unlock()

		// Enviar estado inicial sincronizado
		for i, p := range room.players {
			msg := &pb.GameState{
				RoomCode: room.roomCode,
				Ball:     &pb.Vector{X: 0.5, Y: 0.5},
				Paddle1:  &pb.Vector{X: 0.1, Y: 0.5},
				Paddle2:  &pb.Vector{X: 0.9, Y: 0.5},
				Score1:   0,
				Score2:   0,
				PlayerId: fmt.Sprintf("%d", i+1),
			}
			p.Send(msg)
		}

		// Arrancar físicas
		go room.run()
	}

	// 3) Primer jugador recupera su sala
	streamToRoomMu.Lock()
	room = streamToRoom[stream]
	streamToRoomMu.Unlock()

	// 4) Determinar índice fijo
	room.mu.Lock()
	myIndex := -1
	for i, p := range room.players {
		if p == stream {
			myIndex = i
			break
		}
	}
	room.mu.Unlock()

	// 5) Canal para acciones entrantes
	actions := make(chan *pb.GameAction)
	go func() {
		defer close(actions)
		for {
			a, err := stream.Recv()
			if err != nil {
				return
			}
			actions <- a
		}
	}()

	// 6) Loop principal: procesar acciones
	for {
		action, ok := <-actions
		if !ok {
			// Si se desconecta, quitamos del room
			room.mu.Lock()
			// eliminar stream de room.players
			for idx, p := range room.players {
				if p == stream {
					room.players = append(room.players[:idx], room.players[idx+1:]...)
					break
				}
			}
			room.mu.Unlock()
			return nil
		}
		action.PlayerId = fmt.Sprintf("%d", myIndex+1)
		room.handleAction(action)
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterPingPongServer(grpcServer, &server{})
	reflection.Register(grpcServer)
	log.Println("Servidor gRPC corriendo en :50051")
	grpcServer.Serve(lis)
}
