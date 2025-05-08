package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"google.golang.org/grpc"

	"image/color"

	pb "JuegoCeN/proto"
)

// Game implements ebiten.Game
type Game struct {
	stream   pb.PingPong_PlayClient
	state    *pb.GameState
	updates  chan *pb.GameState
	playerID string
}

// NewGame initializes the game and starts receiving state updates
func NewGame(stream pb.PingPong_PlayClient, initial *pb.GameState, playerID string) *Game {
	g := &Game{
		stream:   stream,
		state:    initial,
		updates:  make(chan *pb.GameState, 1),
		playerID: playerID,
	}
	go g.receiveUpdates()
	return g
}

// receiveUpdates streams game state from server
type receiveError struct{ error }

func (g *Game) receiveUpdates() {
	for {
		st, err := g.stream.Recv()
		if err != nil {
			log.Printf("Error receiving state: %v", err)
			return
		}
		select {
		case g.updates <- st:
		default:
		}
	}
}

// Update is called every tick (1/60s)
func (g *Game) Update() error {
	// apply latest state if available
	select {
	case st := <-g.updates:
		g.state = st
	default:
	}
	// handle input
	move := "NONE"
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		move = "UP"
	} else if ebiten.IsKeyPressed(ebiten.KeyS) {
		move = "DOWN"
	}
	// send action
	action := &pb.GameAction{PlayerId: g.playerID, Move: move}
	if err := g.stream.Send(action); err != nil {
		log.Printf("Error sending action: %v", err)
	}
	return nil
}

// Draw renders the current game state
func (g *Game) Draw(screen *ebiten.Image) {
	w, h := screen.Size()
	// draw ball
	bx := float64(g.state.Ball.X) * float64(w)
	by := float64(g.state.Ball.Y) * float64(h)
	ebitenutil.DrawRect(screen, bx-5, by-5, 10, 10, color.White)
	// draw paddles
	p1y := float64(g.state.Paddle1.Y)*float64(h) - 30
	ebitenutil.DrawRect(screen, 10, p1y, 10, 60, color.White)
	p2y := float64(g.state.Paddle2.Y)*float64(h) - 30
	// right paddle
	ebitenutil.DrawRect(screen, float64(w-20), p2y, 10, 60, color.White)
	// draw simple scores
	ebitenutil.DebugPrintAt(screen,
		fmt.Sprintf("%d", g.state.Score1), w/4, 10)
	ebitenutil.DebugPrintAt(screen,
		fmt.Sprintf("%d", g.state.Score2), 3*w/4, 10)
}

// Layout defines the fixed screen dimensions
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 800, 600
}

func main() {
	// connect to gRPC server
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewPingPongClient(conn)
	// open Play stream
	stream, err := client.Play(context.Background())
	if err != nil {
		log.Fatalf("Play failed: %v", err)
	}
	// receive initial state
	initial, err := stream.Recv()
	if err != nil {
		log.Fatalf("Failed to get initial state: %v", err)
	}
	// choose player ID ("1" or "2")
	playerID := "1"
	game := NewGame(stream, initial, playerID)
	ebiten.SetWindowSize(800, 600)
	ebiten.SetWindowTitle("Ping Pong")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatalf("Game exited with error: %v", err)
	}
}
