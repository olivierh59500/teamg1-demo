package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"log"
	"math"
	"sort"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/ym-player/pkg/stsound"
)

const (
	// Screen dimensions
	screenWidth  = 768
	screenHeight = 540

	// Canvas dimensions
	stCanvasWidth  = 640
	stCanvasHeight = 400

	// Animation parameters
	fadeSpeed     = 0.03
	scrollSpeed   = 4.0
	rotationSpeed = 0.05
	zoomSpeed     = 0.01
	plasmaSpeed   = 0.02

	// Font parameters
	fontHeight     = 36
	fontWidth      = 48 // Average width for font characters
	introFontScale = 2.0
	demoFontScale  = 1.5 // Reduced for better readability
)

// Embedded assets
var (
	//go:embed assets/font.png
	fontData []byte
	//go:embed assets/teamg1_logo.png
	teamG1LogoData []byte
	//go:embed assets/gameone_logo.png
	gameOneLogoData []byte
	//go:embed assets/texture.png
	textureData []byte
	//go:embed assets/music.ym
	musicData []byte
)

// Letter represents a character in the bitmap font
type Letter struct {
	x, y  int
	width int
}

// Vector3 represents a 3D point in space
type Vector3 struct {
	X, Y, Z float64
}

// Face represents a textured quad face
type Face struct {
	P1, P2, P3, P4     int
	UV1, UV2, UV3, UV4 [2]float32 // Texture coordinates
}

// PlasmaField represents the plasma effect background
type PlasmaField struct {
	time   float64
	width  int
	height int
	buffer *ebiten.Image
}

// ScrollChar represents a character in the scrolling text
type ScrollChar struct {
	char  rune
	x     float64
	baseY float64
	waveY float64
	scale float64
	alpha float64
}

// LogoDistortion handles the logo distortion effect
type LogoDistortion struct {
	distSin    []float64
	distCount  int
	distCanvas *ebiten.Image
}

// YMPlayer wraps the YM player for Ebiten audio
type YMPlayer struct {
	player       *stsound.StSound
	sampleRate   int
	buffer       []int16
	mutex        sync.Mutex
	position     int64
	totalSamples int64
	loop         bool
	volume       float64
}

// NewYMPlayer creates a new YM player instance
func NewYMPlayer(data []byte, sampleRate int, loop bool) (*YMPlayer, error) {
	player := stsound.CreateWithRate(sampleRate)

	if err := player.LoadMemory(data); err != nil {
		player.Destroy()
		return nil, fmt.Errorf("failed to load YM data: %w", err)
	}

	player.SetLoopMode(loop)

	info := player.GetInfo()
	totalSamples := int64(info.MusicTimeInMs) * int64(sampleRate) / 1000

	return &YMPlayer{
		player:       player,
		sampleRate:   sampleRate,
		buffer:       make([]int16, 4096),
		totalSamples: totalSamples,
		loop:         loop,
		volume:       1.0,
	}, nil
}

// Read implements io.Reader for audio streaming
func (y *YMPlayer) Read(p []byte) (n int, err error) {
	y.mutex.Lock()
	defer y.mutex.Unlock()

	samplesNeeded := len(p) / 4
	outBuffer := make([]int16, samplesNeeded*2)

	processed := 0
	for processed < samplesNeeded {
		chunkSize := samplesNeeded - processed
		if chunkSize > len(y.buffer) {
			chunkSize = len(y.buffer)
		}

		if !y.player.Compute(y.buffer[:chunkSize], chunkSize) {
			if !y.loop {
				for i := processed * 2; i < len(outBuffer); i++ {
					outBuffer[i] = 0
				}
				err = io.EOF
				break
			}
		}

		for i := 0; i < chunkSize; i++ {
			sample := int16(float64(y.buffer[i]) * y.volume)
			outBuffer[(processed+i)*2] = sample
			outBuffer[(processed+i)*2+1] = sample
		}

		processed += chunkSize
		y.position += int64(chunkSize)
	}

	buf := make([]byte, 0, len(outBuffer)*2)
	for _, sample := range outBuffer {
		buf = append(buf, byte(sample), byte(sample>>8))
	}

	copy(p, buf)
	n = len(buf)
	if n > len(p) {
		n = len(p)
	}

	return n, err
}

// Seek implements io.Seeker
func (y *YMPlayer) Seek(offset int64, whence int) (int64, error) {
	return y.position, nil
}

// Close releases resources
func (y *YMPlayer) Close() error {
	y.mutex.Lock()
	defer y.mutex.Unlock()

	if y.player != nil {
		y.player.Destroy()
		y.player = nil
	}
	return nil
}

// CRT shader with enhanced effects - FIXED with time uniform
const crtShaderSrc = `
package main

var Time float
var ScreenSize vec2

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	var uv vec2
	uv = texCoord
	
	// Enhanced barrel distortion
	var dc vec2
	dc = uv - 0.5
	dc = dc * (1.0 + dot(dc, dc) * 0.25)
	uv = dc + 0.5
	
	// Check bounds
	if uv.x < 0.0 || uv.x > 1.0 || uv.y < 0.0 || uv.y > 1.0 {
		return vec4(0.0, 0.0, 0.0, 1.0)
	}
	
	// Sample texture
	var col vec4
	col = imageSrc0At(uv)
	
	// Scanlines with varying intensity
	var scanline float
	scanline = sin(uv.y * 800.0 + Time * 2.0) * 0.04
	col.rgb = col.rgb - scanline
	
	// RGB shift (chromatic aberration)
	var rShift float
	var bShift float
	rShift = imageSrc0At(uv + vec2(0.003, 0.0)).r
	bShift = imageSrc0At(uv - vec2(0.003, 0.0)).b
	col.r = rShift
	col.b = bShift
	
	// Phosphor glow
	var glow float
	glow = imageSrc0At(uv + vec2(0.001, 0.001)).g * 0.1
	col.g = col.g + glow
	
	// Vignette effect
	var vignette float
	vignette = 1.0 - dot(dc, dc) * 0.7
	col.rgb = col.rgb * vignette
	
	// Flickering
	var flicker float
	flicker = 0.95 + sin(Time * 120.0) * 0.05
	col.rgb = col.rgb * flicker
	
	return col * color
}
`

// Game represents the main demo state
type Game struct {
	// Images
	fontImg     *ebiten.Image
	teamG1Logo  *ebiten.Image
	gameOneLogo *ebiten.Image
	texture     *ebiten.Image

	// Canvases
	stCanvas     *ebiten.Image
	plasmaCanvas *ebiten.Image
	cubeCanvas   *ebiten.Image
	scrollCanvas *ebiten.Image
	logoCanvas   *ebiten.Image

	// Effects
	plasmaField *PlasmaField
	logoDistort *LogoDistortion

	// 3D Textured cube
	cubeVertices []Vector3
	cubeFaces    []Face
	cubeRotation Vector3

	// Logo spiral
	logoPositions []Vector3
	logoTime      float64

	// Scrolling for demo (TCB style)
	scrollText      string
	scrollTextRunes []rune
	scrollX         float64
	scrollOffset    float64
	scrollWave      []float64

	// Intro scrolling
	introScrollText string
	introTextRunes  []rune

	// Animation state
	fadeImg       float64
	pos           float64
	shaderTime    float64
	introComplete bool
	demoTime      float64

	// Audio
	audioContext *audio.Context
	audioPlayer  *audio.Player
	ymPlayer     *YMPlayer

	// Shader
	crtShader *ebiten.Shader

	// Font data
	letterData map[rune]*Letter

	// Intro state
	introX      int
	introLetter int
	introSpeed  int
	surfScroll1 *ebiten.Image
	surfScroll2 *ebiten.Image
	tmpImg      *ebiten.Image

	// Draw options (optimization)
	drawOp     *ebiten.DrawImageOptions
	drawRectOp *ebiten.DrawRectShaderOptions
}

// NewGame creates and initializes a new game instance
func NewGame() *Game {
	g := &Game{
		fadeImg:     2.0,
		letterData:  make(map[rune]*Letter),
		introX:      -1,
		introLetter: -1,
		introSpeed:  int(scrollSpeed),
		drawOp:      &ebiten.DrawImageOptions{},
		drawRectOp:  &ebiten.DrawRectShaderOptions{},
		logoTime:    0,
		scrollWave:  make([]float64, 0),
	}

	// Initialize scrolling texts
	spc := "     "
	g.introScrollText = spc +
		"C'EST MERCREDI..." + spc +
		"JE REPETE, C'EST MERCREDI ET LE MERCREDI..." + spc
	g.introTextRunes = []rune(g.introScrollText)

	// Main demo text
	g.scrollText = spc + spc +
		"C'EST TEAMG1 A 16H00 SUR GAMEONE POUR TOUS LES GAMERS, LES GEEKS ET LES NERDS." + spc +
		"ENCORE UN BON APRES MIDI AVEC TOUTE L'EQUIPE DE TEAMG1! VIVEMENT 16H00" + spc + spc + spc + spc
	g.scrollTextRunes = []rune(g.scrollText)

	// Load images
	g.loadImages()

	// Create canvases
	g.stCanvas = ebiten.NewImage(stCanvasWidth, stCanvasHeight)
	g.plasmaCanvas = ebiten.NewImage(stCanvasWidth/2, stCanvasHeight/2)
	g.cubeCanvas = ebiten.NewImage(stCanvasWidth, stCanvasHeight)
	g.scrollCanvas = ebiten.NewImage(stCanvasWidth+512, int(fontHeight*demoFontScale))
	g.logoCanvas = ebiten.NewImage(stCanvasWidth, stCanvasHeight)

	// For intro, ensure all canvases have consistent sizes
	introScrollHeight := int(fontHeight * introFontScale)
	g.surfScroll1 = ebiten.NewImage(screenWidth, introScrollHeight)
	g.surfScroll2 = ebiten.NewImage(screenWidth, introScrollHeight)
	g.tmpImg = ebiten.NewImage(screenWidth, introScrollHeight)

	// Initialize font data
	g.initFontData()

	// Initialize 3D textured cube
	g.initCube()

	// Initialize logo spiral positions
	g.initLogoSpiral()

	// Initialize plasma effect
	g.plasmaField = &PlasmaField{
		width:  stCanvasWidth / 2,
		height: stCanvasHeight / 2,
		buffer: g.plasmaCanvas,
	}

	// Initialize logo distortion
	g.initLogoDistortion()

	// Initialize audio
	g.initAudio()

	// Compile CRT shader
	var err error
	g.crtShader, err = ebiten.NewShader([]byte(crtShaderSrc))
	if err != nil {
		log.Printf("Failed to compile CRT shader: %v", err)
	}

	return g
}

// initLogoDistortion initializes the logo distortion effect
func (g *Game) initLogoDistortion() {
	g.logoDistort = &LogoDistortion{
		distCanvas: ebiten.NewImage(256, 122),
		distCount:  0,
	}

	// Initialize distortion sine table with more subtle values
	g.logoDistort.distSin = make([]float64, 0)

	// Gentle sine waves for line distortion
	for i := 0; i < 200; i++ {
		g.logoDistort.distSin = append(g.logoDistort.distSin, 50*math.Sin(float64(i)*0.05))
	}

	// Some variation
	for i := 0; i < 100; i++ {
		g.logoDistort.distSin = append(g.logoDistort.distSin, 30*math.Sin(float64(i)*0.1)+20*math.Cos(float64(i)*0.07))
	}

	// Different pattern
	for i := 0; i < 150; i++ {
		g.logoDistort.distSin = append(g.logoDistort.distSin, 40*math.Sin(float64(i)*0.03))
	}

	// Calm section
	for i := 0; i < 100; i++ {
		g.logoDistort.distSin = append(g.logoDistort.distSin, 20*math.Sin(float64(i)*0.08))
	}

	// Near zero
	for i := 0; i < 50; i++ {
		g.logoDistort.distSin = append(g.logoDistort.distSin, 10*math.Sin(float64(i)*0.1))
	}
}

// initFontData initializes the bitmap font character data
func (g *Game) initFontData() {
	data := []struct {
		char  rune
		x, y  int
		width int
	}{
		{' ', 0, 0, 32},
		{'!', 48, 0, 16},
		{'"', 96, 0, 32},
		{'\'', 336, 0, 16},
		{'(', 384, 0, 32},
		{')', 432, 0, 32},
		{'+', 48, 36, 48},
		{',', 96, 36, 16},
		{'-', 144, 36, 32},
		{'.', 192, 36, 16},
		{'0', 288, 36, 48},
		{'1', 336, 36, 48},
		{'2', 384, 36, 48},
		{'3', 432, 36, 48},
		{'4', 0, 72, 48},
		{'5', 48, 72, 48},
		{'6', 96, 72, 48},
		{'7', 144, 72, 48},
		{'8', 192, 72, 48},
		{'9', 240, 72, 48},
		{':', 288, 72, 16},
		{';', 336, 72, 16},
		{'<', 384, 72, 32},
		{'=', 432, 72, 32},
		{'>', 0, 108, 32},
		{'?', 48, 108, 48},
		{'A', 144, 108, 48},
		{'B', 192, 108, 48},
		{'C', 240, 108, 48},
		{'D', 288, 108, 48},
		{'E', 336, 108, 48},
		{'F', 384, 108, 48},
		{'G', 432, 108, 48},
		{'H', 0, 144, 48},
		{'I', 48, 144, 16},
		{'J', 96, 144, 48},
		{'K', 144, 144, 48},
		{'L', 192, 144, 48},
		{'M', 240, 144, 48},
		{'N', 288, 144, 48},
		{'O', 336, 144, 48},
		{'P', 384, 144, 48},
		{'Q', 432, 144, 48},
		{'R', 0, 180, 48},
		{'S', 48, 180, 48},
		{'T', 96, 180, 48},
		{'U', 144, 180, 48},
		{'V', 192, 180, 48},
		{'W', 240, 180, 48},
		{'X', 288, 180, 48},
		{'Y', 336, 180, 48},
		{'Z', 384, 180, 48},
		{'#', 432, 180, 48}, // Special character for logo
	}

	for _, d := range data {
		g.letterData[d.char] = &Letter{
			x:     d.x,
			y:     d.y,
			width: d.width,
		}
	}
}

// initScrollWave()
func (g *Game) initScrollWave() {
	g.scrollWave = make([]float64, 0)

	// First wave pattern
	stp1 := 7.0 / 180.0 * math.Pi
	stp2 := 3.0 / 180.0 * math.Pi
	for i := 0; i < 389; i++ {
		x := 20*math.Sin(float64(i)*stp1) + 30*math.Cos(float64(i)*stp2)
		g.scrollWave = append(g.scrollWave, x)
	}

	// Second wave pattern
	stp1 = 72.0 / 180.0 * math.Pi
	for i := 0; i < 120; i++ {
		x := 4 * math.Sin(float64(i)*stp1)
		g.scrollWave = append(g.scrollWave, x)
	}

	// Third wave pattern
	stp1 = 8.0 / 180.0 * math.Pi
	for i := 0; i < 68; i++ {
		x := 40 * math.Sin(float64(i)*stp1)
		g.scrollWave = append(g.scrollWave, x)
	}
}

// initCube initializes the 3D textured cube
func (g *Game) initCube() {
	// Cube vertices
	size := 100.0
	g.cubeVertices = []Vector3{
		{-size, -size, -size}, // 0
		{size, -size, -size},  // 1
		{size, size, -size},   // 2
		{-size, size, -size},  // 3
		{-size, -size, size},  // 4
		{size, -size, size},   // 5
		{size, size, size},    // 6
		{-size, size, size},   // 7
	}

	// Cube faces with texture coordinates
	g.cubeFaces = []Face{
		{4, 5, 6, 7, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Front
		{1, 0, 3, 2, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Back
		{5, 1, 2, 6, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Right
		{0, 4, 7, 3, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Left
		{7, 6, 2, 3, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Top
		{0, 1, 5, 4, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Bottom
	}
}

// initLogoSpiral initializes positions for the GAMEONE logo spiral
func (g *Game) initLogoSpiral() {
	g.logoPositions = make([]Vector3, 12)
	for i := 0; i < 12; i++ {
		angle := float64(i) * math.Pi * 2 / 12
		radius := 150.0
		g.logoPositions[i] = Vector3{
			X: math.Cos(angle) * radius,
			Y: math.Sin(angle) * radius,
			Z: 0,
		}
	}
}

// loadImages loads all image assets
func (g *Game) loadImages() {
	var err error

	// Load font
	img, _, err := image.Decode(bytes.NewReader(fontData))
	if err != nil {
		log.Printf("Failed to load font: %v", err)
		g.fontImg = ebiten.NewImage(480, 216)
		g.fontImg.Fill(color.White)
	} else {
		g.fontImg = ebiten.NewImageFromImage(img)
	}

	// Load TEAMG1 logo
	img, _, err = image.Decode(bytes.NewReader(teamG1LogoData))
	if err != nil {
		log.Printf("Failed to load TEAMG1 logo: %v", err)
		g.teamG1Logo = ebiten.NewImage(256, 64)
		g.teamG1Logo.Fill(color.RGBA{255, 0, 255, 255})
	} else {
		g.teamG1Logo = ebiten.NewImageFromImage(img)
	}

	// Load GAMEONE logo
	img, _, err = image.Decode(bytes.NewReader(gameOneLogoData))
	if err != nil {
		log.Printf("Failed to load GAMEONE logo: %v", err)
		g.gameOneLogo = ebiten.NewImage(64, 64)
		g.gameOneLogo.Fill(color.RGBA{0, 255, 255, 255})
	} else {
		g.gameOneLogo = ebiten.NewImageFromImage(img)
	}

	// Load texture
	img, _, err = image.Decode(bytes.NewReader(textureData))
	if err != nil {
		log.Printf("Failed to load texture: %v", err)
		g.texture = ebiten.NewImage(256, 256)
		// Create a procedural checkerboard texture
		for y := 0; y < 256; y++ {
			for x := 0; x < 256; x++ {
				if (x/32+y/32)%2 == 0 {
					g.texture.Set(x, y, color.RGBA{255, 0, 255, 255})
				} else {
					g.texture.Set(x, y, color.RGBA{0, 255, 255, 255})
				}
			}
		}
	} else {
		g.texture = ebiten.NewImageFromImage(img)
	}
}

// initAudio initializes the audio system with YM music
func (g *Game) initAudio() {
	g.audioContext = audio.NewContext(44100)

	var err error
	g.ymPlayer, err = NewYMPlayer(musicData, 44100, true)
	if err != nil {
		log.Printf("Failed to create YM player: %v", err)
		return
	}

	g.audioPlayer, err = g.audioContext.NewPlayer(g.ymPlayer)
	if err != nil {
		log.Printf("Failed to create audio player: %v", err)
		g.ymPlayer.Close()
		g.ymPlayer = nil
		return
	}

	g.audioPlayer.SetVolume(0.7)
}

// updatePlasma updates the plasma effect
func (g *Game) updatePlasma() {
	g.plasmaField.time += plasmaSpeed

	// Generate plasma pattern
	for y := 0; y < g.plasmaField.height; y++ {
		for x := 0; x < g.plasmaField.width; x++ {
			// Multiple sine waves for complex patterns
			v1 := math.Sin(float64(x)*0.02 + g.plasmaField.time)
			v2 := math.Sin(float64(y)*0.03 + g.plasmaField.time*1.5)
			v3 := math.Sin(math.Sqrt(float64(x*x+y*y))*0.01 + g.plasmaField.time*0.5)
			v4 := math.Sin((float64(x)*0.01 + float64(y)*0.01) + g.plasmaField.time*2)

			v := (v1 + v2 + v3 + v4) / 4

			// Map to color
			r := uint8((math.Sin(v*math.Pi) + 1) * 127)
			green := uint8((math.Sin(v*math.Pi+2*math.Pi/3) + 1) * 127)
			b := uint8((math.Sin(v*math.Pi+4*math.Pi/3) + 1) * 127)

			g.plasmaField.buffer.Set(x, y, color.RGBA{r, green, b, 255})
		}
	}
}

// animIntro handles intro animation
func (g *Game) animIntro() {
	if g.introX < 0 {
		if g.introLetter >= 0 {
			char := g.getIntroLetter(g.introLetter)
			if letter, ok := g.letterData[char]; ok {
				g.introX += int(float64(letter.width) * introFontScale)
			}
		}
		g.introLetter++
		if g.introLetter >= len(g.introTextRunes) {
			g.introComplete = true
			g.fadeImg = 0
			return
		}
	}
	g.introX -= 6 // Faster speed

	// Scroll temporary canvas - IMPORTANT: clear first to avoid trails
	g.surfScroll2.Clear()
	srcRect := image.Rect(6, 0, g.surfScroll1.Bounds().Dx(), int(fontHeight*introFontScale))
	g.drawOp.GeoM.Reset()
	g.drawOp.ColorScale.Reset()
	g.surfScroll2.DrawImage(g.surfScroll1.SubImage(srcRect).(*ebiten.Image), g.drawOp)

	// IMPORTANT: Clear surfScroll1 before drawing to avoid trails
	g.surfScroll1.Clear()
	g.surfScroll1.DrawImage(g.surfScroll2, g.drawOp)

	// Draw new letter
	char := g.getIntroLetter(g.introLetter)
	if letter, ok := g.letterData[char]; ok {
		srcRect := image.Rect(letter.x, letter.y, letter.x+letter.width, letter.y+fontHeight)
		g.drawOp.GeoM.Reset()
		g.drawOp.ColorScale.Reset() // Reset color scale
		g.drawOp.GeoM.Scale(introFontScale, introFontScale)
		g.drawOp.GeoM.Translate(float64(stCanvasWidth+g.introX), 0)
		g.surfScroll1.DrawImage(g.fontImg.SubImage(srcRect).(*ebiten.Image), g.drawOp)
	}

	g.shaderTime += 0.016
}

// getIntroLetter gets intro letter at position
func (g *Game) getIntroLetter(pos int) rune {
	if len(g.introTextRunes) == 0 {
		return ' '
	}
	char := g.introTextRunes[pos%len(g.introTextRunes)]

	// Convert lowercase to uppercase since the font only has uppercase
	if char >= 'a' && char <= 'z' {
		char = char - 'a' + 'A'
	}

	// Debug: log if 'I' is being processed
	if char == 'I' {
		if _, ok := g.letterData[char]; !ok {
			log.Printf("Warning: 'I' not found in letterData!")
		}
	}

	return char
}

// drawTexturedCube draws the 3D textured cube
func (g *Game) drawTexturedCube() {
	g.cubeCanvas.Clear()

	// Update rotation
	g.cubeRotation.X += 0.02
	g.cubeRotation.Y += 0.03
	g.cubeRotation.Z += 0.01

	// Transform vertices
	transformedVertices := make([]Vector3, len(g.cubeVertices))
	for i, v := range g.cubeVertices {
		// Apply rotation
		x := v.X
		y := v.Y
		z := v.Z

		// Rotate X
		y2 := y*math.Cos(g.cubeRotation.X) - z*math.Sin(g.cubeRotation.X)
		z2 := y*math.Sin(g.cubeRotation.X) + z*math.Cos(g.cubeRotation.X)
		y = y2
		z = z2

		// Rotate Y
		x2 := x*math.Cos(g.cubeRotation.Y) + z*math.Sin(g.cubeRotation.Y)
		z2 = -x*math.Sin(g.cubeRotation.Y) + z*math.Cos(g.cubeRotation.Y)
		x = x2
		z = z2

		// Rotate Z
		x2 = x*math.Cos(g.cubeRotation.Z) - y*math.Sin(g.cubeRotation.Z)
		y2 = x*math.Sin(g.cubeRotation.Z) + y*math.Cos(g.cubeRotation.Z)

		transformedVertices[i] = Vector3{X: x2, Y: y2, Z: z2}
	}

	// Sort faces by depth
	type faceDepth struct {
		face  Face
		depth float64
	}

	faces := make([]faceDepth, len(g.cubeFaces))
	for i, face := range g.cubeFaces {
		avgZ := (transformedVertices[face.P1].Z + transformedVertices[face.P2].Z +
			transformedVertices[face.P3].Z + transformedVertices[face.P4].Z) / 4.0
		faces[i] = faceDepth{face: face, depth: avgZ}
	}

	sort.Slice(faces, func(i, j int) bool {
		return faces[i].depth < faces[j].depth
	})

	// Draw faces
	centerX := float32(g.cubeCanvas.Bounds().Dx() / 2)
	centerY := float32(g.cubeCanvas.Bounds().Dy() / 2)
	fov := 300.0

	for _, fd := range faces {
		face := fd.face

		// Project vertices
		var screenPoints [4][2]float32
		for i, p := range []int{face.P1, face.P2, face.P3, face.P4} {
			v := transformedVertices[p]
			scale := fov / (fov + v.Z + 300)
			screenPoints[i][0] = centerX + float32(v.X*scale)
			screenPoints[i][1] = centerY + float32(v.Y*scale)
		}

		// Check if face is visible (backface culling)
		v1x := screenPoints[1][0] - screenPoints[0][0]
		v1y := screenPoints[1][1] - screenPoints[0][1]
		v2x := screenPoints[2][0] - screenPoints[0][0]
		v2y := screenPoints[2][1] - screenPoints[0][1]

		if v1x*v2y-v1y*v2x < 0 {
			continue
		}

		// Draw textured quad
		vertices := []ebiten.Vertex{
			{
				DstX: screenPoints[0][0], DstY: screenPoints[0][1],
				SrcX:   face.UV1[0] * float32(g.texture.Bounds().Dx()),
				SrcY:   face.UV1[1] * float32(g.texture.Bounds().Dy()),
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
			},
			{
				DstX: screenPoints[1][0], DstY: screenPoints[1][1],
				SrcX:   face.UV2[0] * float32(g.texture.Bounds().Dx()),
				SrcY:   face.UV2[1] * float32(g.texture.Bounds().Dy()),
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
			},
			{
				DstX: screenPoints[2][0], DstY: screenPoints[2][1],
				SrcX:   face.UV3[0] * float32(g.texture.Bounds().Dx()),
				SrcY:   face.UV3[1] * float32(g.texture.Bounds().Dy()),
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
			},
			{
				DstX: screenPoints[3][0], DstY: screenPoints[3][1],
				SrcX:   face.UV4[0] * float32(g.texture.Bounds().Dx()),
				SrcY:   face.UV4[1] * float32(g.texture.Bounds().Dy()),
				ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
			},
		}

		indices := []uint16{0, 1, 2, 0, 2, 3}

		op := &ebiten.DrawTrianglesOptions{}
		g.cubeCanvas.DrawTriangles(vertices, indices, g.texture, op)
	}
}

// drawLogoSpiral draws the GAMEONE logos in a spiral pattern
func (g *Game) drawLogoSpiral() {
	g.logoCanvas.Clear()

	g.logoTime += 0.02

	for i, pos := range g.logoPositions {
		// Rotate position
		angle := g.logoTime + float64(i)*math.Pi*2/12
		x := math.Cos(angle) * math.Sqrt(pos.X*pos.X+pos.Y*pos.Y)
		y := math.Sin(angle) * math.Sqrt(pos.X*pos.X+pos.Y*pos.Y)

		// Add wave motion
		x += math.Sin(g.logoTime*2+float64(i)) * 20
		y += math.Cos(g.logoTime*2+float64(i)) * 20

		// Scale based on position
		scale := 0.5 + 0.5*math.Sin(g.logoTime+float64(i)*0.5)

		// Draw logo
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-float64(g.gameOneLogo.Bounds().Dx())/2, -float64(g.gameOneLogo.Bounds().Dy())/2)
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(x+float64(g.logoCanvas.Bounds().Dx())/2, y+float64(g.logoCanvas.Bounds().Dy())/2)

		g.logoCanvas.DrawImage(g.gameOneLogo, op)
	}
}

// drawDistortedLogo draws the TEAMG1 logo with sine wave distortion (like JS version)
func (g *Game) drawDistortedLogo() {
	// Update distortion counter
	g.logoDistort.distCount += 2 // Moderate speed

	// Base position - this will move across the screen
	baseX := float64(g.stCanvas.Bounds().Dx()) / 2
	logoY := 60.0

	// Calculate overall logo movement (can move across full screen width)
	overallMovement := math.Sin(float64(g.logoDistort.distCount)*0.01) * float64(g.stCanvas.Bounds().Dx()/2)

	// Apply distortion per scanline with reduced amplitude
	for y := 0; y < g.teamG1Logo.Bounds().Dy(); y++ {
		// Get distortion value for this line - reduced amplitude
		idx := (g.logoDistort.distCount + y*2) % len(g.logoDistort.distSin)
		lineDistortion := g.logoDistort.distSin[idx] * 0.15 // Much smaller line distortion

		// Calculate final X position
		finalX := baseX + overallMovement + lineDistortion - float64(g.teamG1Logo.Bounds().Dx())/2

		// Wrap around screen edges
		screenWidth := float64(g.stCanvas.Bounds().Dx())
		logoWidth := float64(g.teamG1Logo.Bounds().Dx())

		// Draw this line of the logo
		srcRect := image.Rect(0, y, g.teamG1Logo.Bounds().Dx(), y+1)

		// Main position
		if finalX > -logoWidth && finalX < screenWidth {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(finalX, logoY+float64(y))
			g.stCanvas.DrawImage(g.teamG1Logo.SubImage(srcRect).(*ebiten.Image), op)
		}

		// Draw wrapped portion if needed
		if finalX < 0 {
			// Logo is partially off left, draw wrapped portion on right
			wrapX := screenWidth + finalX
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(wrapX, logoY+float64(y))
			g.stCanvas.DrawImage(g.teamG1Logo.SubImage(srcRect).(*ebiten.Image), op)
		} else if finalX+logoWidth > screenWidth {
			// Logo is partially off right, draw wrapped portion on left
			wrapX := finalX - screenWidth
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(wrapX, logoY+float64(y))
			g.stCanvas.DrawImage(g.teamG1Logo.SubImage(srcRect).(*ebiten.Image), op)
		}
	}
}

// drawScrollText draws the scrolling text TCB-Replicants style
func (g *Game) drawScrollText() {
	// Initialize wave if empty
	if len(g.scrollWave) == 0 {
		g.initScrollWave()
	}

	// Clear scroll canvas
	g.scrollCanvas.Clear()

	// Update scroll position
	g.scrollX += 2.0

	// Calculate total text width
	totalWidth := 0.0
	for _, char := range g.scrollTextRunes {
		if letter, ok := g.letterData[char]; ok {
			totalWidth += float64(letter.width) * demoFontScale
		} else {
			totalWidth += 32 * demoFontScale
		}
	}

	// Reset when scrolled completely off
	if g.scrollX >= totalWidth {
		g.scrollX = 0
	}

	// IMPORTANT: Draw text starting from canvas edge, not screen edge
	// The canvas is wider than the screen to allow for wave distortion
	startX := float64(g.scrollCanvas.Bounds().Dx()) - g.scrollX
	xPos := startX

	for _, char := range g.scrollTextRunes {
		if letter, ok := g.letterData[char]; ok {
			// Draw character if potentially visible
			if xPos > -200 && xPos < float64(g.scrollCanvas.Bounds().Dx())+200 {
				srcRect := image.Rect(letter.x, letter.y, letter.x+letter.width, letter.y+fontHeight)
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(demoFontScale, demoFontScale)
				op.GeoM.Translate(xPos, 0)
				g.scrollCanvas.DrawImage(g.fontImg.SubImage(srcRect).(*ebiten.Image), op)
			}
			xPos += float64(letter.width) * demoFontScale
		} else {
			xPos += 32 * demoFontScale
		}
	}

	// Apply horizontal wave distortion line by line
	baseY := float64(g.stCanvas.Bounds().Dy()) - 100
	scrollHeight := int(fontHeight * demoFontScale)

	// Update wave offset
	g.scrollOffset += 0.5

	// Draw each line with horizontal offset
	waveIndex := int(g.scrollOffset)

	// The key is to draw from the scroll canvas to the screen canvas
	// taking into account that the text position in scrollCanvas is different
	for y := 0; y < scrollHeight/2; y++ {
		// Get wave offset for this line
		idx := (waveIndex + y) % len(g.scrollWave)
		offsetX := g.scrollWave[idx]

		// Calculate source position - this is the key fix
		// We need to sample from the right part of the scroll canvas
		srcX := int(offsetX) + 64 + (g.scrollCanvas.Bounds().Dx()-g.stCanvas.Bounds().Dx())/2

		// Source rectangle from scroll canvas
		srcRect := image.Rect(srcX, y*2, srcX+g.stCanvas.Bounds().Dx(), (y+1)*2)

		// Ensure we stay within bounds
		if srcRect.Min.X < 0 {
			srcRect.Min.X = 0
		}
		if srcRect.Max.X > g.scrollCanvas.Bounds().Dx() {
			srcRect.Max.X = g.scrollCanvas.Bounds().Dx()
		}

		if srcRect.Min.X < srcRect.Max.X && srcRect.Dx() > 0 {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(0, baseY+float64(y*2))

			g.stCanvas.DrawImage(g.scrollCanvas.SubImage(srcRect).(*ebiten.Image), op)
		}
	}
}

// drawMainDemo draws the main demo scene
func (g *Game) drawMainDemo() {
	// Update effects
	g.updatePlasma()
	g.demoTime += 0.016

	// Clear main canvas
	g.stCanvas.Fill(color.Black)

	// Draw plasma background (scaled up)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(2, 2)
	g.stCanvas.DrawImage(g.plasmaCanvas, op)

	// Draw textured cube
	g.drawTexturedCube()
	op = &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(0.8)
	g.stCanvas.DrawImage(g.cubeCanvas, op)

	// Draw distorted TEAMG1 logo
	g.drawDistortedLogo()

	// Draw scrolling text
	g.drawScrollText()

	// Draw logo spiral
	g.drawLogoSpiral()
	op = &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(0.6)
	g.stCanvas.DrawImage(g.logoCanvas, op)

}

// Update updates the game state
func (g *Game) Update() error {
	// Handle fullscreen toggle
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}

	if !g.introComplete {
		g.animIntro()
	} else {
		// Fade in main scene
		if g.fadeImg < 1 {
			g.fadeImg += fadeSpeed
			if g.fadeImg > 1 {
				g.fadeImg = 1
			}
		}

		// Start music when demo begins
		if g.fadeImg > 0.1 && g.audioPlayer != nil && !g.audioPlayer.IsPlaying() {
			g.audioPlayer.Play()
		}

		// Update main demo
		g.pos += 0.01
	}

	return nil
}

// Draw renders the game
func (g *Game) Draw(screen *ebiten.Image) {
	if !g.introComplete {
		// Draw intro
		screen.Fill(color.Black)

		// Draw the intro scroll with or without shader at fixed Y position
		yPos := screenHeight/2 - int(fontHeight*introFontScale)/2

		if g.crtShader != nil {
			// Create a temporary image at the exact position needed
			tempImg := ebiten.NewImage(screenWidth, int(fontHeight*introFontScale))
			tempImg.DrawImage(g.surfScroll1, nil)

			g.drawRectOp.Images[0] = tempImg
			g.drawRectOp.GeoM.Reset()
			g.drawRectOp.GeoM.Translate(0, float64(yPos))
			g.drawRectOp.Uniforms = map[string]interface{}{
				"Time":       float32(g.shaderTime),
				"ScreenSize": []float32{float32(screenWidth), float32(screenHeight)},
			}

			screen.DrawRectShader(screenWidth, int(fontHeight*introFontScale), g.crtShader, g.drawRectOp)
		} else {
			// Fallback without shader - draw at fixed position
			g.drawOp.GeoM.Reset()
			g.drawOp.GeoM.Translate(0, float64(yPos))
			screen.DrawImage(g.surfScroll1, g.drawOp)
		}

	} else {
		// Draw main demo
		screen.Fill(color.Black)
		g.drawMainDemo()

		// Final composite with fade - center the canvas
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(64, 70)
		op.ColorScale.ScaleAlpha(float32(g.fadeImg))
		screen.DrawImage(g.stCanvas, op)
	}
}

// Layout returns the screen dimensions
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

// Cleanup releases resources
func (g *Game) Cleanup() {
	if g.audioPlayer != nil {
		g.audioPlayer.Close()
	}
	if g.ymPlayer != nil {
		g.ymPlayer.Close()
	}
	if g.crtShader != nil {
		g.crtShader.Dispose()
	}
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("TEAMG1 Demo - A Tribute to the Golden Age")

	game := NewGame()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}

	game.Cleanup()
}
