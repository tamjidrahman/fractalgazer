package main

import (
	"fmt"
	"image/color"
	"runtime"
	"sync"

	"math/cmplx"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	screenWidth  = 1920
	screenHeight = 1440
	MAX_ITER     = 100
)

func getColor(index int) color.Color {
	if index == MAX_ITER {
		return color.Black
	}

	t := float64(index) / float64(MAX_ITER)
	r := uint8(9 * (1 - t) * t * t * t * 255)
	g := uint8(15 * (1 - t) * (1 - t) * t * t * 255)
	b := uint8(8.5 * (1 - t) * (1 - t) * (1 - t) * t * 255)

	return color.RGBA{r, g, b, 255}
}

type Game struct {
	fps   float64
	xMin  float64
	xMax  float64
	yMin  float64
	yMax  float64
	scale float64
}
type Coord = float64
type PixelCoord = int

func (g *Game) Update() error {
	g.fps = ebiten.ActualFPS()

	if ebiten.IsKeyPressed(ebiten.KeyA) {
		g.xMin -= 0.1 * g.scale
		g.xMax -= 0.1 * g.scale
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		g.xMin += 0.1 * g.scale
		g.xMax += 0.1 * g.scale
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		g.yMin += 0.1 * g.scale
		g.yMax += 0.1 * g.scale
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		g.yMin -= 0.1 * g.scale
		g.yMax -= 0.1 * g.scale
	}
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		g.scale *= 0.95
		yMid := (g.yMin + g.yMax) / 2
		xMid := (g.xMin + g.xMax) / 2
		g.xMin = xMid - g.scale
		g.xMax = xMid + g.scale
		g.yMin = yMid - g.scale
		g.yMax = yMid + g.scale
	}
	if ebiten.IsKeyPressed(ebiten.KeyE) {
		g.scale *= 1.05
		yMid := (g.yMin + g.yMax) / 2
		xMid := (g.xMin + g.xMax) / 2
		g.xMin = xMid - g.scale
		g.xMax = xMid + g.scale
		g.yMin = yMid - g.scale
		g.yMax = yMid + g.scale
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.White)
	colorCanvas(g, screen)

	ebitenutil.DebugPrint(screen, fmt.Sprintf("FPS: %.2f\nZoom: %.2f\nCenter: (%.2f, %.2f)", g.fps, 1/g.scale, (g.xMin+g.xMax)/2, (g.yMin+g.yMax)/2))
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

type Point struct {
	X, Y Coord
}

type PixelPoint struct {
	X, Y PixelCoord
}

func (p *Point) toPixelPoint(g *Game) PixelPoint {
	return PixelPoint{
		X: int((p.X - g.xMin) / (g.xMax - g.xMin) * float64(screenWidth)),
		Y: int((g.yMax - p.Y) / (g.yMax - g.yMin) * float64(screenHeight)),
	}
}

func (p *PixelPoint) toPoint(g *Game) Point {
	return Point{
		X: g.xMin + (g.xMax-g.xMin)/screenWidth*float64(p.X),
		Y: g.yMax - (g.yMax-g.yMin)/screenHeight*float64(p.Y),
	}
}

func getTValue(p *Point) int {

	z_orig := complex(p.X, p.Y)

	z := complex(p.X, p.Y)
	iterations := 1

	for ; cmplx.Abs(complex128(z)) < 2 && iterations < MAX_ITER; iterations++ {
		z = z*z + z_orig
	}

	return iterations
}

var wg sync.WaitGroup

var t_values = [screenWidth][screenHeight]int{}

func getTValueChunked(g *Game, startY, endY int) {
	defer wg.Done()
	for x := 0; x < screenWidth; x++ {
		for y := startY; y < endY; y++ {
			p := PixelPoint{X: x, Y: y}
			point := p.toPoint(g)
			t_value := getTValue(&point)
			t_values[x][y] = t_value
		}
	}
}

func colorCanvas(g *Game, screen *ebiten.Image) {
	numGoroutines := runtime.NumCPU()
	chunkSize := screenHeight / numGoroutines

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go getTValueChunked(g, i*chunkSize, (i+1)*chunkSize)
	}

	wg.Wait()

	for x := 0; x < screenWidth; x++ {
		for y := 0; y < screenHeight; y++ {
			screen.Set(x, y, getColor(t_values[x][y]))
		}
	}
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Mandelbrot Set")
	game := &Game{
		xMin:  -2,
		xMax:  2,
		yMin:  -2,
		yMax:  2,
		scale: 2,
	}
	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
