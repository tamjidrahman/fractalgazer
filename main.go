package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	screenWidth  = 1920
	screenHeight = 1080
	MAX_ITER     = 300
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
	cacheShift  int
	recording   bool
	frameCount  int
	zoomPath    []Point
	pathIndex   int
	aspectRatio float64
}

func (g *Game) generateZoomPath() {
	startPoint := Point{X: -0.743643887037158704752191506114774, Y: 0.131825904205311970493132056385139}
	steps := 10000
	g.zoomPath = make([]Point, steps)
	for i := 0; i < steps; i++ {
		g.zoomPath[i] = Point{X: startPoint.X, Y: startPoint.Y}
	}
}

type Coord = float64
type PixelCoord = int

func (g *Game) Update() error {
	g.fps = ebiten.ActualFPS()

	if ebiten.IsKeyPressed(ebiten.KeyR) {
		if !g.recording {
			g.recording = true
			g.frameCount = 0
			g.pathIndex = 0
			g.generateZoomPath()
		} else {
			g.recording = false
		}
	}

	if g.recording && g.pathIndex < len(g.zoomPath) {
		target := g.zoomPath[g.pathIndex]
		g.xMin = target.X - g.scale
		g.xMax = target.X + g.scale
		yScale := g.scale / g.aspectRatio
		g.yMin = target.Y - yScale
		g.yMax = target.Y + yScale
		g.scale *= 0.99 // Adjust this value to control zoom speed
		g.cacheValid = false

		if err := g.saveFrame(); err != nil {
			return err
		}
		g.frameCount++
		g.pathIndex++

		if g.pathIndex == len(g.zoomPath) {
			g.recording = false
		}
	}

	if !g.recording {
		viewChanged := false

		if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyLeft) {
			g.xMin -= g.linearStep * g.scale
			g.xMax -= g.linearStep * g.scale
			g.shiftCache(1)
			viewChanged = true
		}
		if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyRight) {
			g.xMin += g.linearStep * g.scale
			g.xMax += g.linearStep * g.scale
			g.shiftCache(-1)
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
		if ebiten.IsKeyPressed(ebiten.KeyQ) || ebiten.IsKeyPressed(ebiten.KeyE) {
			if ebiten.IsKeyPressed(ebiten.KeyQ) {
				g.scale *= (1.0 - g.zoomSpeed)
			} else {
				g.scale *= (1.0 + g.zoomSpeed)
			}
			yMid := (g.yMin + g.yMax) / 2
			xMid := (g.xMin + g.xMax) / 2
			g.xMin = xMid - g.scale
			g.xMax = xMid + g.scale
			yScale := g.scale / g.aspectRatio
			g.yMin = yMid - yScale
			g.yMax = yMid + yScale
			viewChanged = true
		}

		if viewChanged {
			g.cacheValid = false
		}
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

func (g *Game) shiftCache(direction int) {
	g.cacheShift += direction
	if abs(g.cacheShift) >= screenWidth {
		g.cacheValid = false
		g.cacheShift = 0
		return
	}

	if direction > 0 {
		for y := 0; y < screenHeight; y++ {
			for x := screenWidth - 1; x >= direction; x-- {
				g.tValueCache[x][y] = g.tValueCache[x-direction][y]
			}
		}
	} else {
		for y := 0; y < screenHeight; y++ {
			for x := 0; x < screenWidth+direction; x++ {
				g.tValueCache[x][y] = g.tValueCache[x-direction][y]
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
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
		X: g.xMin + (g.xMax-g.xMin)/float64(screenWidth)*float64(p.X),
		Y: g.yMax - (g.yMax-g.yMin)/float64(screenHeight)*float64(p.Y),
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
		g.cacheShift = 0
	} else if g.cacheShift != 0 {
		startX := 0
		endX := abs(g.cacheShift)
		if g.cacheShift < 0 {
			startX = screenWidth + g.cacheShift
			endX = screenWidth
		}

		for y := 0; y < screenHeight; y++ {
			for x := startX; x < endX; x++ {
				point := Point{
					X: g.xMin + (g.xMax-g.xMin)/screenWidth*float64(x),
					Y: g.yMax - (g.yMax-g.yMin)/screenHeight*float64(y),
				}
				g.tValueCache[x][y] = getTValue(&point)
			}
		}
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

func (g *Game) saveFrame() error {
	bounds := image.Rect(0, 0, screenWidth, screenHeight)
	img := image.NewRGBA(bounds)
	for y := 0; y < screenHeight; y++ {
		for x := 0; x < screenWidth; x++ {
			v := g.tValueCache[x][y]
			color := getColor(v)
			img.Set(x, y, color)
		}
	}

	os.MkdirAll("frames", os.ModePerm)
	filename := filepath.Join("frames", fmt.Sprintf("frame_%04d.png", g.frameCount))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

func main() {
	headless := flag.Bool("headless", false, "Run in headless mode for recording")
	flag.Parse()

	aspectRatio := float64(screenWidth) / float64(screenHeight)

	game := &Game{
		xMin:        -2,
		xMax:        2,
		yMin:        -2 / aspectRatio,
		yMax:        2 / aspectRatio,
		scale:       2,
		linearStep:  0.1,
		zoomSpeed:   0.05,
		aspectRatio: aspectRatio,
	}
	game.generateZoomPath()

	if *headless {
		runHeadless(game)
	} else {
		ebiten.SetWindowSize(screenWidth, screenHeight)
		ebiten.SetWindowTitle("Mandelbrot Set")
		if err := ebiten.RunGame(game); err != nil {
			panic(err)
		}
	}
}

func runHeadless(game *Game) {
	fmt.Println("Running in headless mode for recording...")
	game.recording = true
	game.frameCount = 0
	game.pathIndex = 0

	for game.pathIndex < len(game.zoomPath) {
		game.Update()
		img := ebiten.NewImage(screenWidth, screenHeight)
		game.Draw(img)
		if err := game.saveFrame(); err != nil {
			panic(err)
		}
		fmt.Printf("Saved frame %d\r", game.frameCount)
	}
	fmt.Println("\nRecording complete!")
}
