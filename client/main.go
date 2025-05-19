package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
	"google.golang.org/grpc"

	pb "JuegoCeN/proto"
)

type State int

const (
	StateMenu State = iota
	StateWaiting
	StatePlaying
	StateOpponentLeft
)

type Button struct {
	label      string
	x, y, w, h float64
	onClick    func()
}

type Game struct {
	client      pb.PingPongClient
	conn        *grpc.ClientConn
	stream      pb.PingPong_PlayClient
	state       State
	updates     chan *pb.GameState
	errChan     chan error
	menuBg      *ebiten.Image
	gameBg      *ebiten.Image
	button      Button
	gameState   *pb.GameState
	playerID    string
	joiningDone bool
	leftAt      time.Time
	lastUpdate  time.Time
}

func NewGame(client pb.PingPongClient, conn *grpc.ClientConn) *Game {
	menuImg, _, err := ebitenutil.NewImageFromFile("client/robot.png")
	if err != nil {
		log.Fatalf("No se pudo cargar robot.png: %v", err)
	}
	gameImg, _, err := ebitenutil.NewImageFromFile("client/fondo.png")
	if err != nil {
		log.Fatalf("No se pudo cargar fondo.png: %v", err)
	}

	g := &Game{
		client:     client,
		conn:       conn,
		state:      StateMenu,
		menuBg:     menuImg,
		gameBg:     gameImg,
		lastUpdate: time.Now(),
	}

	g.button = Button{
		label: "Unirse a una partida",
		x:     300, y: 280, w: 200, h: 50,
		onClick: func() {
			g.state = StateWaiting
			g.joiningDone = false
			g.gameState = nil
			g.playerID = ""
			g.updates = make(chan *pb.GameState, 1)
			g.errChan = make(chan error, 1)
			// abrir stream
			stream, err := g.client.Play(context.Background())
			if err != nil {
				log.Printf("No se pudo abrir Play: %v", err)
				g.state = StateMenu
				return
			}
			g.stream = stream
			go g.receiveUpdates()
		},
	}

	return g
}

func (g *Game) receiveUpdates() {
	for {
		st, err := g.stream.Recv()
		if err != nil {
			g.errChan <- err
			return
		}
		g.lastUpdate = time.Now()
		select {
		case g.updates <- st:
		default:
		}
	}
}

func (g *Game) Update() error {
	switch g.state {
	case StateMenu:
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			x, y := ebiten.CursorPosition()
			if float64(x) >= g.button.x && float64(x) <= g.button.x+g.button.w &&
				float64(y) >= g.button.y && float64(y) <= g.button.y+g.button.h {
				g.button.onClick()
			}
		}

	case StateWaiting:
		select {
		case err := <-g.errChan:
			log.Printf("Error de stream en espera: %v", err)
			g.state = StateOpponentLeft
			g.leftAt = time.Now()
			return nil
		case st := <-g.updates:
			g.gameState = st
			g.playerID = st.PlayerId
			g.state = StatePlaying
			return nil
		default:
			if !g.joiningDone {
				g.stream.Send(&pb.GameAction{RoomCode: ""})
				g.joiningDone = true
			}
		}

	case StatePlaying:
		if time.Since(g.lastUpdate) > 2*time.Second {
			g.state = StateOpponentLeft
			g.leftAt = time.Now()
			return nil
		}

		select {
		case err := <-g.errChan:
			log.Printf("Error de stream en juego: %v", err)
			g.state = StateOpponentLeft
			g.leftAt = time.Now()
			return nil
		case st := <-g.updates:
			g.gameState = st
		default:
		}

		if g.gameState != nil {
			move := "NONE"
			if ebiten.IsKeyPressed(ebiten.KeyW) {
				move = "UP"
			} else if ebiten.IsKeyPressed(ebiten.KeyS) {
				move = "DOWN"
			}
			g.stream.Send(&pb.GameAction{
				PlayerId: g.playerID,
				Move:     move,
				RoomCode: "",
			})
		}

	case StateOpponentLeft:
		if time.Since(g.leftAt) > 3*time.Second {
			if g.stream != nil {
				g.stream.CloseSend()
			}
			g.state = StateMenu
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Definiciones que coinciden con el servidor:
	const (
		paddleW  = 10.0
		paddleH  = 80.0
		ballRad  = 8.0
		ballSize = ballRad * 2
		margin   = 10.0
	)

	switch g.state {
	case StateMenu:
		screen.DrawImage(g.menuBg, nil)
		ebitenutil.DrawRect(screen, g.button.x, g.button.y, g.button.w, g.button.h,
			color.RGBA{100, 100, 200, 255})
		text.Draw(screen, g.button.label, basicfont.Face7x13,
			int(g.button.x+20), int(g.button.y+30), color.White)

	case StateWaiting:
		screen.DrawImage(g.menuBg, nil)
		w, h := screen.Size()
		ebitenutil.DrawRect(screen, 0, 0, float64(w), float64(h),
			color.RGBA{0, 0, 0, 180})
		msg := "Esperando jugador..."
		textWidth := len(msg) * 7
		text.Draw(screen, msg, basicfont.Face7x13,
			(w-textWidth)/2, h/2, color.White)

	case StatePlaying:
		screen.DrawImage(g.gameBg, nil)
		if g.gameState != nil {
			w, h := screen.Size()

			// Bola
			bx := float64(g.gameState.Ball.X) * float64(w)
			by := float64(g.gameState.Ball.Y) * float64(h)
			ebitenutil.DrawRect(screen,
				bx-ballRad, by-ballRad,
				ballSize, ballSize,
				color.White,
			)

			// Pala izquierda
			p1y := float64(g.gameState.Paddle1.Y) * float64(h)
			ebitenutil.DrawRect(screen,
				margin, p1y-paddleH/2,
				paddleW, paddleH,
				color.White,
			)

			// Pala derecha
			p2y := float64(g.gameState.Paddle2.Y) * float64(h)
			ebitenutil.DrawRect(screen,
				float64(w)-margin-paddleW, p2y-paddleH/2,
				paddleW, paddleH,
				color.White,
			)

			// Marcador
			text.Draw(screen, fmt.Sprintf("%d", g.gameState.Score1),
				basicfont.Face7x13, w/4, 20, color.White)
			text.Draw(screen, fmt.Sprintf("%d", g.gameState.Score2),
				basicfont.Face7x13, 3*w/4, 20, color.White)
		}

	case StateOpponentLeft:
		screen.DrawImage(g.menuBg, nil)
		w, h := screen.Size()
		ebitenutil.DrawRect(screen, 0, 0, float64(w), float64(h),
			color.RGBA{0, 0, 0, 180})
		msg := "El oponente abandonó la partida"
		textWidth := len(msg) * 7
		text.Draw(screen, msg, basicfont.Face7x13,
			(w-textWidth)/2, h/2, color.White)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 800, 600
}

func main() {
	// Conectar gRPC
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()
	client := pb.NewPingPongClient(conn)

	game := NewGame(client, conn)
	ebiten.SetWindowSize(800, 600)
	ebiten.SetWindowTitle("Ping Pong Multijugador")
	ebiten.SetRunnableOnUnfocused(true)
	if err := ebiten.RunGame(game); err != nil {
		log.Fatalf("Game exited: %v", err)
	}
}
