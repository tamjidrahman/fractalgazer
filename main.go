package main

import (
	"fmt"
	"image/color"
	"runtime"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	screenWidth  = 1920
	screenHeight = 1440
	MAX_ITER     = 100
)

var colorCache [MAX_ITER + 1]color.RGBA

func init() {
	for i := 0; i <= MAX_ITER; i++ {
		if i == MAX_ITER {
			colorCache[i] = color.RGBA{0, 0, 0, 255}
		} else {
			t := float64(i) / float64(MAX_ITER)
			r := uint8(9 * (1 - t) * t * t * t * 255)
			g := uint8(15 * (1 - t) * (1 - t) * t * t * 255)
			b := uint8(8.5 * (1 - t) * (1 - t) * (1 - t) * t * 255)
			colorCache[i] = color.RGBA{r, g, b, 255}
		}
	}
}

func getColor(index int) color.RGBA {
	return colorCache[index]
}

type Game struct {
	fps         float64
	xMin        float64
	xMax        float64
	yMin        float64
	yMax        float64
	scale       float64
	linearStep  float64
	zoomSpeed   float64
	tValueCache [screenWidth][screenHeight]int
	cacheValid  bool
}
type Coord = float64
type PixelCoord = int

func (g *Game) Update() error {
	g.fps = ebiten.ActualFPS()

	viewChanged := false

	if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyLeft) {
		g.xMin -= g.linearStep * g.scale
		g.xMax -= g.linearStep * g.scale
		viewChanged = true
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyRight) {
		g.xMin += g.linearStep * g.scale
		g.xMax += g.linearStep * g.scale
		viewChanged = true
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyUp) {
		g.yMin += g.linearStep * g.scale
		g.yMax += g.linearStep * g.scale
		viewChanged = true
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyDown) {
		g.yMin -= g.linearStep * g.scale
		g.yMax -= g.linearStep * g.scale
		viewChanged = true
	}
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		g.scale *= (1.0 - g.zoomSpeed)
		yMid := (g.yMin + g.yMax) / 2
		xMid := (g.xMin + g.xMax) / 2
		g.xMin = xMid - g.scale
		g.xMax = xMid + g.scale
		g.yMin = yMid - g.scale
		g.yMax = yMid + g.scale
		viewChanged = true
	}
	if ebiten.IsKeyPressed(ebiten.KeyE) {
		g.scale *= (1.0 + g.zoomSpeed)
		yMid := (g.yMin + g.yMax) / 2
		xMid := (g.xMin + g.xMax) / 2
		g.xMin = xMid - g.scale
		g.xMax = xMid + g.scale
		g.yMin = yMid - g.scale
		g.yMax = yMid + g.scale
		viewChanged = true
	}

	if viewChanged {
		g.cacheValid = false
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
	x, y := p.X, p.Y
	x2, y2 := x*x, y*y
	iteration := 0
	for x2+y2 <= 4 && iteration < MAX_ITER {
		y = 2*x*y + p.Y
		x = x2 - y2 + p.X
		x2, y2 = x*x, y*y
		iteration++

		// Early escape optimization
		if x2+y2 > 4 {
			break
		}
	}
	return iteration
}

var wg sync.WaitGroup

var t_values = [screenWidth][screenHeight]int{}

func getTValueChunked(g *Game, startY, endY int) {
	defer wg.Done()
	for y := startY; y < endY; y++ {
		for x := 0; x < screenWidth; x++ {
			point := Point{
				X: g.xMin + (g.xMax-g.xMin)/screenWidth*float64(x),
				Y: g.yMax - (g.yMax-g.yMin)/screenHeight*float64(y),
			}

			g.tValueCache[x][y] = getTValue(&point)
		}
	}
}

func colorCanvas(g *Game, screen *ebiten.Image) {
	if !g.cacheValid {
		numGoroutines := runtime.NumCPU()
		chunkSize := screenHeight / numGoroutines

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go getTValueChunked(g, i*chunkSize, (i+1)*chunkSize)
		}

		wg.Wait()
		g.cacheValid = true
	}

	pixels := make([]byte, screenWidth*screenHeight*4)
	for y := 0; y < screenHeight; y++ {
		for x := 0; x < screenWidth; x++ {
			v := g.tValueCache[x][y]
			color := getColor(v)
			i := (y*screenWidth + x) * 4
			pixels[i] = color.R
			pixels[i+1] = color.G
			pixels[i+2] = color.B
			pixels[i+3] = color.A
		}
	}

	screen.WritePixels(pixels)
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Mandelbrot Set")
	game := &Game{
		xMin:       -2,
		xMax:       2,
		yMin:       -2,
		yMax:       2,
		scale:      2,
		linearStep: 0.1,
		zoomSpeed:  0.05,
	}
	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
