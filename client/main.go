package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
	"google.golang.org/grpc"

	pb "JuegoCeN/proto"
)

type GameState int

const (
	StateMenu GameState = iota
	StateWaitingRoom
	StateJoinRoom
	StateLeaderboard
	StatePlaying
)

type Button struct {
	label      string
	x, y, w, h float64
	onClick    func()
}

type Game struct {
	state       GameState
	stream      pb.PingPong_PlayClient
	updates     chan *pb.GameState
	playerID    string
	gameState   *pb.GameState
	buttons     []Button
	roomCode    string
	bgImage     *ebiten.Image
	inputCode   string
	joiningDone bool
}

func NewGame() *Game {
	img, _, err := ebitenutil.NewImageFromFile("robot.png")
	if err != nil {
		log.Fatalf("No se pudo cargar la imagen de fondo: %v", err)
	}
	g := &Game{
		state:   StateMenu,
		updates: make(chan *pb.GameState, 1),
		bgImage: img,
	}
	g.initMenu()
	return g
}

func (g *Game) initMenu() {
	centerX := 400.0
	startY := 250.0
	buttonW := 200.0
	buttonH := 50.0
	gap := 70.0

	g.buttons = []Button{
		{"Crear Sala", centerX - buttonW/2, startY, buttonW, buttonH, func() {
			g.state = StateWaitingRoom
			g.roomCode = generateRoomCode()
		}},
		{"Unirse a Sala", centerX - buttonW/2, startY + gap, buttonW, buttonH, func() {
			g.state = StateJoinRoom
			g.inputCode = ""
			g.joiningDone = false
		}},
		{"Tabla de Posiciones", centerX - buttonW/2, startY + 2*gap, buttonW, buttonH, func() {
			g.state = StateLeaderboard
		}},
	}
}

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

func (g *Game) Update() error {
	if g.state == StateMenu && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		for _, btn := range g.buttons {
			if float64(x) >= btn.x && float64(x) <= btn.x+btn.w && float64(y) >= btn.y && float64(y) <= btn.y+btn.h {
				btn.onClick()
			}
		}
		return nil
	}

	if g.state == StateJoinRoom && !g.joiningDone {
		for _, key := range ebiten.InputChars() {
			if key == '\n' || key == '\r' {
				g.roomCode = g.inputCode
				g.state = StatePlaying
				g.joiningDone = true
			} else if key == 8 || key == 127 {
				if len(g.inputCode) > 0 {
					g.inputCode = g.inputCode[:len(g.inputCode)-1]
				}
			} else if len(g.inputCode) < 4 && (key >= 'A' && key <= 'Z' || key >= 'a' && key <= 'z') {
				g.inputCode += string(key)
			}
		}
	}

	if g.state == StatePlaying {
		select {
		case st := <-g.updates:
			g.gameState = st
		default:
		}
		move := "NONE"
		if ebiten.IsKeyPressed(ebiten.KeyS) {
			move = "UP"
		} else if ebiten.IsKeyPressed(ebiten.KeyW) {
			move = "DOWN"
		}
		action := &pb.GameAction{PlayerId: g.playerID, Move: move}
		if err := g.stream.Send(action); err != nil {
			log.Printf("Error sending action: %v", err)
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.bgImage != nil {
		screen.DrawImage(g.bgImage, nil)
	}

	switch g.state {
	case StateMenu:
		for _, btn := range g.buttons {
			ebitenutil.DrawRect(screen, btn.x, btn.y, btn.w, btn.h, color.RGBA{100, 100, 200, 255})
			text.Draw(screen, btn.label, basicfont.Face7x13, int(btn.x+20), int(btn.y+30), color.White)
		}
	case StateWaitingRoom:
		ebitenutil.DrawRect(screen, 180, 240, 440, 140, color.RGBA{0, 0, 0, 200})
		text.Draw(screen, "Esperando a que alguien se una a la sala...", basicfont.Face7x13, 200, 300, color.White)
		text.Draw(screen, fmt.Sprintf("Codigo: %s", g.roomCode), basicfont.Face7x13, 300, 340, color.White)
	case StateJoinRoom:
		ebitenutil.DrawRect(screen, 180, 240, 440, 180, color.RGBA{0, 0, 0, 200})
		text.Draw(screen, "Introduce el codigo de la sala:", basicfont.Face7x13, 200, 270, color.White)
		// botón INGRESAR
		btnX, btnY, btnW, btnH := 320.0, 370.0, 160.0, 30.0
		ebitenutil.DrawRect(screen, btnX, btnY, btnW, btnH, color.RGBA{100, 100, 200, 255})
		text.Draw(screen, "INGRESAR", basicfont.Face7x13, int(btnX+30), int(btnY+20), color.White)
		x, y := ebiten.CursorPosition()
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && float64(x) >= btnX && float64(x) <= btnX+btnW && float64(y) >= btnY && float64(y) <= btnY+btnH {
			if len(g.inputCode) == 4 {
				g.roomCode = g.inputCode
				g.joiningDone = true
				g.state = StatePlaying
			} else {
				text.Draw(screen, "Error: Código inválido", basicfont.Face7x13, 250, 420, color.RGBA{255, 0, 0, 255})
			}
		}
		text.Draw(screen, "Presiona Enter para aceptar", basicfont.Face7x13, 260, 350, color.White)
		text.Draw(screen, g.inputCode, basicfont.Face7x13, 360, 300, color.White)
		text.Draw(screen, "Presiona Enter para aceptar", basicfont.Face7x13, 260, 350, color.White)
	case StateLeaderboard:
		text.Draw(screen, "Tabla de Posiciones (por implementar)", basicfont.Face7x13, 100, 300, color.White)
	case StatePlaying:
		if g.gameState == nil {
			return
		}
		w, h := screen.Size()
		bx := float64(g.gameState.Ball.X) * float64(w)
		by := float64(g.gameState.Ball.Y) * float64(h)
		ebitenutil.DrawRect(screen, bx-5, by-5, 10, 10, color.White)
		p1y := float64(g.gameState.Paddle1.Y)*float64(h) - 30
		ebitenutil.DrawRect(screen, 10, p1y, 10, 60, color.White)
		p2y := float64(g.gameState.Paddle2.Y)*float64(h) - 30
		ebitenutil.DrawRect(screen, float64(w-20), p2y, 10, 60, color.White)
		text.Draw(screen, fmt.Sprintf("%d", g.gameState.Score1), basicfont.Face7x13, w/4, 20, color.White)
		text.Draw(screen, fmt.Sprintf("%d", g.gameState.Score2), basicfont.Face7x13, 3*w/4, 20, color.White)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 800, 600
}

func main() {
	playerID := flag.String("id", "1", "Player ID: \"1\" or \"2\"")
	flag.Parse()

	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewPingPongClient(conn)
	stream, err := client.Play(context.Background())
	if err != nil {
		log.Fatalf("Play failed: %v", err)
	}
	initial, err := stream.Recv()
	if err != nil {
		log.Fatalf("Failed to get initial state: %v", err)
	}

	game := NewGame()
	game.stream = stream
	game.gameState = initial
	game.playerID = *playerID
	go game.receiveUpdates()

	ebiten.SetWindowSize(800, 600)
	ebiten.SetWindowTitle("Ping Pong Multijugador")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatalf("Game exited with error: %v", err)
	}
}

func generateRoomCode() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	code := strings.Builder{}
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 4; i++ {
		code.WriteByte(letters[rand.Intn(len(letters))])
	}
	return code.String()
}
