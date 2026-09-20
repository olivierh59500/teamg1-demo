// Package teamg1demo implements the TEAMG1 demoscene tribute.
package teamg1demo

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"log"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/ym-player/pkg/stsound"
)

const (
	// Screen dimensions
	screenWidth     = 768
	screenHeight    = 540
	maxLogicalWidth = 1280
	sampleRate      = 48000

	// Canvas dimensions
	stCanvasWidth  = 640
	stCanvasHeight = 400

	// Animation parameters
	fadeSpeed   = 0.03
	plasmaSpeed = 0.02

	// Font parameters
	fontHeight     = 36
	introFontScale = 2.0
	demoFontScale  = 1.5 // Reduced for better readability
	logoCount      = 12
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
	width int
	image *ebiten.Image
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
	time        float64
	width       int
	height      int
	buffer      *ebiten.Image
	pixels      []byte
	xSin        []float64
	xCos        []float64
	ySin        []float64
	yCos        []float64
	radialSin   []float64
	radialCos   []float64
	diagonalSin []float64
	diagonalCos []float64
	xWave       []float64
	yWave       []float64
	dirty       bool
}

// LogoDistortion handles the logo distortion effect
type LogoDistortion struct {
	distSin   []float64
	distCount int
}

type faceDepth struct {
	faceIndex int
	depth     float64
}

type scrollGlyph struct {
	image *ebiten.Image
	width float64
}

// YMPlayer wraps the YM player for Ebiten audio
type YMPlayer struct {
	player *stsound.StSound
	buffer []int16
	mutex  sync.Mutex
	loop   bool
}

// NewYMPlayer creates a new YM player instance
func NewYMPlayer(data []byte, sampleRate int, loop bool) (*YMPlayer, error) {
	player := stsound.CreateWithRate(sampleRate)

	if err := player.LoadMemory(data); err != nil {
		player.Destroy()
		return nil, fmt.Errorf("failed to load YM data: %w", err)
	}

	player.SetLoopMode(loop)

	return &YMPlayer{
		player: player,
		buffer: make([]int16, 4096),
		loop:   loop,
	}, nil
}

// Read implements io.Reader for audio streaming
func (y *YMPlayer) Read(p []byte) (n int, err error) {
	y.mutex.Lock()
	defer y.mutex.Unlock()

	if y.player == nil {
		return 0, io.EOF
	}

	samplesNeeded := len(p) / 4
	processed := 0
	for processed < samplesNeeded {
		chunkSize := samplesNeeded - processed
		if chunkSize > len(y.buffer) {
			chunkSize = len(y.buffer)
		}

		if !y.player.Compute(y.buffer[:chunkSize], chunkSize) {
			if !y.loop {
				clear(p[processed*4 : samplesNeeded*4])
				err = io.EOF
				break
			}
		}

		for i := 0; i < chunkSize; i++ {
			sample := y.buffer[i] / 2
			offset := (processed + i) * 4
			p[offset] = byte(sample)
			p[offset+1] = byte(sample >> 8)
			p[offset+2] = byte(sample)
			p[offset+3] = byte(sample >> 8)
		}

		processed += chunkSize
	}

	return samplesNeeded * 4, err
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
	scrollRenderer *scrolling.Scrolling
	stripBatch     *composite.QuadBatch
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
	cubeVertices        [8]Vector3
	cubeFaces           [6]Face
	transformedVertices [8]Vector3
	faceDepths          [6]faceDepth
	cubeDrawVertices    [4]ebiten.Vertex
	cubeDrawIndices     [6]uint16
	cubeTrianglesOpt    ebiten.DrawTrianglesOptions
	cubeRotation        Vector3

	// Logo spiral
	logoPhases [logoCount]float64
	logoTime   float64

	// Scrolling for demo (TCB style)
	scrollGlyphs       []scrollGlyph
	scrollTextWidth    float64
	scrollX            float64
	scrollOffset       float64
	scrollWave         []float64
	scrollVertices     []ebiten.Vertex
	scrollIndices      []uint16
	scrollTrianglesOpt ebiten.DrawTrianglesOptions

	// Intro scrolling
	introTextRunes []rune

	// Animation state
	fadeImg       float64
	shaderTime    float64
	introComplete bool

	// Audio
	audioContext *audio.Context
	audioPlayer  *audio.Player
	ymPlayer     *YMPlayer
	audioReady   bool
	musicStarted bool

	// Shader
	crtShader   *ebiten.Shader
	crtUniforms map[string]any

	// Font data
	letterData      map[rune]Letter
	teamG1LogoLines []*ebiten.Image

	// Intro state
	introX          int
	introLetter     int
	surfScroll1     *ebiten.Image
	surfScroll2     *ebiten.Image
	introShiftImage *ebiten.Image

	// Draw options (optimization)
	drawOp     ebiten.DrawImageOptions
	drawRectOp ebiten.DrawRectShaderOptions
}

// NewGame creates and initializes a new game instance
func NewGame() *Game {
	g := &Game{
		fadeImg:        2.0,
		letterData:     make(map[rune]Letter, 48),
		introX:         -1,
		introLetter:    -1,
		crtUniforms:    map[string]any{"Time": float32(0)},
		scrollVertices: make([]ebiten.Vertex, 0, int(fontHeight*demoFontScale/2)*4),
		scrollIndices:  make([]uint16, 0, int(fontHeight*demoFontScale/2)*6),
	}

	// Initialize scrolling texts
	spc := "     "
	introScrollText := spc +
		"C'EST MERCREDI..." + spc +
		"JE REPETE, C'EST MERCREDI ET LE MERCREDI..." + spc
	g.introTextRunes = []rune(introScrollText)

	// Main demo text
	scrollText := spc + spc +
		"C'EST TEAMG1 A 16H00 SUR GAMEONE POUR TOUS LES GAMERS, LES GEEKS ET LES NERDS." + spc +
		"ENCORE UN BON APRES MIDI AVEC TOUTE L'EQUIPE DE TEAMG1! VIVEMENT 16H00" + spc + spc + spc + spc

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
	g.introShiftImage = g.surfScroll1.SubImage(
		image.Rect(6, 0, screenWidth, introScrollHeight),
	).(*ebiten.Image)

	// Initialize font data
	g.initFontData()
	g.initScrollText([]rune(scrollText))
	g.initScrollWave()
	g.cacheLogoLines()

	// Initialize 3D textured cube
	g.initCube()

	// Initialize logo spiral positions
	g.initLogoSpiral()

	// Initialize plasma effect
	g.plasmaField = newPlasmaField(g.plasmaCanvas)

	// Initialize logo distortion
	g.initLogoDistortion()

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
		distSin: make([]float64, 0, 600),
	}

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
		rect := image.Rect(d.x, d.y, d.x+d.width, d.y+fontHeight)
		g.letterData[d.char] = Letter{
			width: d.width,
			image: g.fontImg.SubImage(rect).(*ebiten.Image),
		}
	}
}

func (g *Game) initScrollText(text []rune) {
	g.scrollGlyphs = make([]scrollGlyph, 0, len(text))
	for _, char := range text {
		letter, ok := g.letterData[char]
		if !ok {
			width := 32 * demoFontScale
			g.scrollGlyphs = append(g.scrollGlyphs, scrollGlyph{width: width})
			g.scrollTextWidth += width
			continue
		}

		width := float64(letter.width) * demoFontScale
		g.scrollGlyphs = append(g.scrollGlyphs, scrollGlyph{
			image: letter.image,
			width: width,
		})
		g.scrollTextWidth += width
	}
}

func (g *Game) cacheLogoLines() {
	height := g.teamG1Logo.Bounds().Dy()
	width := g.teamG1Logo.Bounds().Dx()
	g.teamG1LogoLines = make([]*ebiten.Image, height)
	for y := range height {
		g.teamG1LogoLines[y] = g.teamG1Logo.SubImage(
			image.Rect(0, y, width, y+1),
		).(*ebiten.Image)
	}
}

func (g *Game) initScrollWave() {
	g.scrollWave = make([]float64, 0, 577)

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

func newPlasmaField(buffer *ebiten.Image) *PlasmaField {
	width := buffer.Bounds().Dx()
	height := buffer.Bounds().Dy()
	p := newPlasmaFieldForSize(width, height)
	p.buffer = buffer
	return p
}

func newPlasmaFieldForSize(width, height int) *PlasmaField {
	pixelCount := width * height
	diagonalCount := width + height - 1
	p := &PlasmaField{
		width:       width,
		height:      height,
		pixels:      make([]byte, pixelCount*4),
		xSin:        make([]float64, width),
		xCos:        make([]float64, width),
		ySin:        make([]float64, height),
		yCos:        make([]float64, height),
		radialSin:   make([]float64, pixelCount),
		radialCos:   make([]float64, pixelCount),
		diagonalSin: make([]float64, diagonalCount),
		diagonalCos: make([]float64, diagonalCount),
		xWave:       make([]float64, width),
		yWave:       make([]float64, height),
		dirty:       true,
	}

	for x := range width {
		p.xSin[x], p.xCos[x] = math.Sincos(float64(x) * 0.02)
	}
	for y := range height {
		p.ySin[y], p.yCos[y] = math.Sincos(float64(y) * 0.03)
	}
	for diagonal := range diagonalCount {
		p.diagonalSin[diagonal], p.diagonalCos[diagonal] = math.Sincos(float64(diagonal) * 0.01)
	}
	for y := range height {
		row := y * width
		for x := range width {
			phase := math.Sqrt(float64(x*x+y*y)) * 0.01
			p.radialSin[row+x], p.radialCos[row+x] = math.Sincos(phase)
		}
	}

	return p
}

func (p *PlasmaField) advance() {
	p.time += plasmaSpeed
	p.dirty = true
}

func plasmaColor(value float64) byte {
	value = (value + 1) * 127
	if value <= 0 {
		return 0
	}
	if value >= 254 {
		return 254
	}
	return byte(value)
}

func (p *PlasmaField) updatePixels() {
	sinTime, cosTime := math.Sincos(p.time)
	sinTime15, cosTime15 := math.Sincos(p.time * 1.5)
	sinTime05, cosTime05 := math.Sincos(p.time * 0.5)
	sinTime2, cosTime2 := math.Sincos(p.time * 2)

	for x := range p.width {
		p.xWave[x] = p.xSin[x]*cosTime + p.xCos[x]*sinTime
	}
	for y := range p.height {
		p.yWave[y] = p.ySin[y]*cosTime15 + p.yCos[y]*sinTime15
	}

	const sinTwoPiThird = 0.8660254037844386
	for y := range p.height {
		row := y * p.width
		for x := range p.width {
			index := row + x
			radial := p.radialSin[index]*cosTime05 + p.radialCos[index]*sinTime05
			diagonal := p.diagonalSin[x+y]*cosTime2 + p.diagonalCos[x+y]*sinTime2
			value := (p.xWave[x] + p.yWave[y] + radial + diagonal) / 4
			sinColor, cosColor := math.Sincos(value * math.Pi)

			pixel := index * 4
			p.pixels[pixel] = plasmaColor(sinColor)
			p.pixels[pixel+1] = plasmaColor(-0.5*sinColor + sinTwoPiThird*cosColor)
			p.pixels[pixel+2] = plasmaColor(-0.5*sinColor - sinTwoPiThird*cosColor)
			p.pixels[pixel+3] = 0xff
		}
	}
}

func (p *PlasmaField) draw() {
	if !p.dirty {
		return
	}
	p.updatePixels()
	p.buffer.WritePixels(p.pixels)
	p.dirty = false
}

// initCube initializes the 3D textured cube
func (g *Game) initCube() {
	// Cube vertices
	size := 100.0
	g.cubeVertices = [8]Vector3{
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
	g.cubeFaces = [6]Face{
		{4, 5, 6, 7, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Front
		{1, 0, 3, 2, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Back
		{5, 1, 2, 6, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Right
		{0, 4, 7, 3, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Left
		{7, 6, 2, 3, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Top
		{0, 1, 5, 4, [2]float32{0, 0}, [2]float32{1, 0}, [2]float32{1, 1}, [2]float32{0, 1}}, // Bottom
	}
	g.cubeDrawIndices = [6]uint16{0, 1, 2, 0, 2, 3}
	for i := range g.cubeDrawVertices {
		g.cubeDrawVertices[i].ColorR = 1
		g.cubeDrawVertices[i].ColorG = 1
		g.cubeDrawVertices[i].ColorB = 1
		g.cubeDrawVertices[i].ColorA = 1
	}
}

// initLogoSpiral initializes positions for the GAMEONE logo spiral
func (g *Game) initLogoSpiral() {
	for i := range logoCount {
		g.logoPhases[i] = float64(i) * math.Pi * 2 / logoCount
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
	g.audioContext = audio.NewContext(sampleRate)

	ymPlayer, err := NewYMPlayer(musicData, sampleRate, true)
	if err != nil {
		log.Printf("Failed to create YM player: %v", err)
		return
	}
	g.ymPlayer = ymPlayer

	audioPlayer, err := g.audioContext.NewPlayer(ymPlayer)
	if err != nil {
		log.Printf("Failed to create audio player: %v", err)
		if closeErr := ymPlayer.Close(); closeErr != nil {
			log.Printf("Failed to close YM player: %v", closeErr)
		}
		g.ymPlayer = nil
		return
	}
	g.audioPlayer = audioPlayer
}

func (g *Game) startMusic() {
	if g.musicStarted || g.audioPlayer == nil {
		return
	}
	g.audioPlayer.Play()
	g.musicStarted = true
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
	g.drawOp.GeoM.Reset()
	g.drawOp.ColorScale.Reset()
	g.surfScroll2.DrawImage(g.introShiftImage, &g.drawOp)

	// IMPORTANT: Clear surfScroll1 before drawing to avoid trails
	g.surfScroll1.Clear()
	g.surfScroll1.DrawImage(g.surfScroll2, &g.drawOp)

	// Draw new letter
	char := g.getIntroLetter(g.introLetter)
	if letter, ok := g.letterData[char]; ok {
		g.drawOp.GeoM.Reset()
		g.drawOp.ColorScale.Reset() // Reset color scale
		g.drawOp.GeoM.Scale(introFontScale, introFontScale)
		g.drawOp.GeoM.Translate(float64(stCanvasWidth+g.introX), 0)
		g.surfScroll1.DrawImage(letter.image, &g.drawOp)
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

	return char
}

// drawTexturedCube draws the 3D textured cube
func (g *Game) drawTexturedCube() {
	g.cubeCanvas.Clear()

	sinX, cosX := math.Sincos(g.cubeRotation.X)
	sinY, cosY := math.Sincos(g.cubeRotation.Y)
	sinZ, cosZ := math.Sincos(g.cubeRotation.Z)

	// Transform vertices
	for i, v := range g.cubeVertices {
		x := v.X
		y := v.Y
		z := v.Z

		y2 := y*cosX - z*sinX
		z2 := y*sinX + z*cosX
		y = y2
		z = z2

		x2 := x*cosY + z*sinY
		z2 = -x*sinY + z*cosY
		x = x2

		x2 = x*cosZ - y*sinZ
		y2 = x*sinZ + y*cosZ

		g.transformedVertices[i] = Vector3{X: x2, Y: y2, Z: z2}
	}

	for i, face := range g.cubeFaces {
		g.faceDepths[i] = faceDepth{
			faceIndex: i,
			depth: (g.transformedVertices[face.P1].Z + g.transformedVertices[face.P2].Z +
				g.transformedVertices[face.P3].Z + g.transformedVertices[face.P4].Z) / 4,
		}
	}
	for i := 1; i < len(g.faceDepths); i++ {
		item := g.faceDepths[i]
		j := i
		for j > 0 && item.depth < g.faceDepths[j-1].depth {
			g.faceDepths[j] = g.faceDepths[j-1]
			j--
		}
		g.faceDepths[j] = item
	}

	// Draw faces
	centerX := float32(g.cubeCanvas.Bounds().Dx() / 2)
	centerY := float32(g.cubeCanvas.Bounds().Dy() / 2)
	textureWidth := float32(g.texture.Bounds().Dx())
	textureHeight := float32(g.texture.Bounds().Dy())
	const fov = 300.0

	for _, fd := range g.faceDepths {
		face := g.cubeFaces[fd.faceIndex]

		// Project vertices
		var screenPoints [4][2]float32
		pointIndices := [4]int{face.P1, face.P2, face.P3, face.P4}
		for i, pointIndex := range pointIndices {
			v := g.transformedVertices[pointIndex]
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

		uvs := [4][2]float32{face.UV1, face.UV2, face.UV3, face.UV4}
		for i := range g.cubeDrawVertices {
			g.cubeDrawVertices[i].DstX = screenPoints[i][0]
			g.cubeDrawVertices[i].DstY = screenPoints[i][1]
			g.cubeDrawVertices[i].SrcX = uvs[i][0] * textureWidth
			g.cubeDrawVertices[i].SrcY = uvs[i][1] * textureHeight
		}

		g.cubeCanvas.DrawTriangles(
			g.cubeDrawVertices[:],
			g.cubeDrawIndices[:],
			g.texture,
			&g.cubeTrianglesOpt,
		)
	}
}

// drawLogoSpiral draws the GAMEONE logos in a spiral pattern
func (g *Game) drawLogoSpiral() {
	g.logoCanvas.Clear()

	for i, phase := range g.logoPhases {
		// Rotate position
		angle := g.logoTime + phase
		x := math.Cos(angle) * 150
		y := math.Sin(angle) * 150

		// Add wave motion
		x += math.Sin(g.logoTime*2+float64(i)) * 20
		y += math.Cos(g.logoTime*2+float64(i)) * 20

		// Scale based on position
		scale := 0.5 + 0.5*math.Sin(g.logoTime+float64(i)*0.5)

		// Draw logo
		var op ebiten.DrawImageOptions
		op.GeoM.Translate(-float64(g.gameOneLogo.Bounds().Dx())/2, -float64(g.gameOneLogo.Bounds().Dy())/2)
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(x+float64(g.logoCanvas.Bounds().Dx())/2, y+float64(g.logoCanvas.Bounds().Dy())/2)

		composite.Instance{Image: g.gameOneLogo, Options: op}.Draw(g.logoCanvas)
	}
}

// drawDistortedLogo draws the TEAMG1 logo with sine wave distortion (like JS version)
func (g *Game) drawDistortedLogo() {
	// Base position - this will move across the screen
	canvasWidth := float64(g.stCanvas.Bounds().Dx())
	logoWidth := float64(g.teamG1Logo.Bounds().Dx())
	baseX := canvasWidth / 2
	const logoY = 60.0

	// Calculate overall logo movement (can move across full screen width)
	overallMovement := math.Sin(float64(g.logoDistort.distCount)*0.01) * canvasWidth / 2

	// Apply distortion per scanline with reduced amplitude
	for y, logoLine := range g.teamG1LogoLines {
		// Get distortion value for this line - reduced amplitude
		idx := (g.logoDistort.distCount + y*2) % len(g.logoDistort.distSin)
		lineDistortion := g.logoDistort.distSin[idx] * 0.15 // Much smaller line distortion

		// Calculate final X position
		finalX := baseX + overallMovement + lineDistortion - logoWidth/2

		// Main position
		if finalX > -logoWidth && finalX < canvasWidth {
			var op ebiten.DrawImageOptions
			op.GeoM.Translate(finalX, logoY+float64(y))
			g.stCanvas.DrawImage(logoLine, &op)
		}

		// Draw wrapped portion if needed
		if finalX < 0 {
			// Logo is partially off left, draw wrapped portion on right
			wrapX := canvasWidth + finalX
			var op ebiten.DrawImageOptions
			op.GeoM.Translate(wrapX, logoY+float64(y))
			g.stCanvas.DrawImage(logoLine, &op)
		} else if finalX+logoWidth > canvasWidth {
			// Logo is partially off right, draw wrapped portion on left
			wrapX := finalX - canvasWidth
			var op ebiten.DrawImageOptions
			op.GeoM.Translate(wrapX, logoY+float64(y))
			g.stCanvas.DrawImage(logoLine, &op)
		}
	}
}

// drawScrollText draws the scrolling text TCB-Replicants style
func (g *Game) drawScrollText() {
	g.scrollCanvas.Clear()
	canvasWidth := g.scrollCanvas.Bounds().Dx()
	if g.scrollRenderer == nil {
		glyphs := make([]scrolling.Glyph, len(g.scrollGlyphs))
		for i, gl := range g.scrollGlyphs {
			glyphs[i] = scrolling.Glyph{Image: gl.image, Advance: gl.width, ScaleX: demoFontScale, ScaleY: demoFontScale}
		}
		var err error
		g.scrollRenderer, err = scrolling.New(scrolling.Config{Glyphs: glyphs})
		if err != nil {
			panic(err)
		}
		g.stripBatch = composite.NewQuadBatch(int(fontHeight*demoFontScale) / 2)
		g.stripBatch.AlternateDiagonal = true
	}
	state := scrolling.IdentityState()
	state.X = float64(canvasWidth) - g.scrollX
	state.Map = func(s scrolling.Sample, op *ebiten.DrawImageOptions) bool {
		return s.X < float64(canvasWidth+200) && s.X+s.Glyph.Advance > -200
	}
	g.scrollRenderer.DrawAt(g.scrollCanvas, state)
	baseY := float64(g.stCanvas.Bounds().Dy()) - 100
	waveIndex := int(g.scrollOffset)
	g.stripBatch.Options = g.scrollTrianglesOpt
	g.stripBatch.Begin(g.stCanvas, g.scrollCanvas)
	for y := 0; y < int(fontHeight*demoFontScale)/2; y++ {
		offsetX := g.scrollWave[(waveIndex+y)%len(g.scrollWave)]
		x0 := int(offsetX) + 64 + (canvasWidth-g.stCanvas.Bounds().Dx())/2
		x1 := x0 + g.stCanvas.Bounds().Dx()
		x0 = max(0, x0)
		x1 = min(canvasWidth, x1)
		if x0 >= x1 {
			continue
		}
		g.stripBatch.Rect(image.Rect(x0, y*2, x1, y*2+2), 0, float32(baseY+float64(y*2)), float32(x1-x0), 2)
	}
	g.stripBatch.Flush()
}

// drawMainDemo draws the main demo scene
func (g *Game) drawMainDemo() {
	g.stCanvas.Fill(color.Black)
	g.plasmaField.draw()

	// Draw plasma background (scaled up)
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(2, 2)
	g.stCanvas.DrawImage(g.plasmaCanvas, &op)

	// Draw textured cube
	g.drawTexturedCube()
	op = ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(0.8)
	g.stCanvas.DrawImage(g.cubeCanvas, &op)

	// Draw distorted TEAMG1 logo
	g.drawDistortedLogo()

	// Draw scrolling text
	g.drawScrollText()

	// Draw logo spiral
	g.drawLogoSpiral()
	op = ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(0.6)
	g.stCanvas.DrawImage(g.logoCanvas, &op)
}

func (g *Game) advanceMainDemo() {
	g.plasmaField.advance()
	g.cubeRotation.X += 0.02
	g.cubeRotation.Y += 0.03
	g.cubeRotation.Z += 0.01
	g.logoTime += 0.02
	g.logoDistort.distCount += 2

	g.scrollX += 2
	if g.scrollX >= g.scrollTextWidth {
		g.scrollX = 0
	}
	g.scrollOffset += 0.5
	if g.scrollOffset >= float64(len(g.scrollWave)) {
		g.scrollOffset -= float64(len(g.scrollWave))
	}
}

// Update updates the game state
func (g *Game) Update() error {
	// mobile.SetGame constructs the game before Android has installed its
	// application context. Opening audio on the first tick avoids blocking the
	// native-library initialization path.
	if !g.audioReady {
		g.audioReady = true
		g.initAudio()
	}

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

		if g.fadeImg > 0.1 {
			g.startMusic()
		}

		g.advanceMainDemo()
	}

	return nil
}

// Draw renders the game
func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)
	offsetX := (screen.Bounds().Dx() - screenWidth) / 2

	if !g.introComplete {
		// Draw the intro scroll with or without shader at fixed Y position
		yPos := screenHeight/2 - int(fontHeight*introFontScale)/2

		if g.crtShader != nil {
			g.drawRectOp.Images[0] = g.surfScroll1
			g.drawRectOp.GeoM.Reset()
			g.drawRectOp.GeoM.Translate(float64(offsetX), float64(yPos))
			g.crtUniforms["Time"] = float32(g.shaderTime)
			g.drawRectOp.Uniforms = g.crtUniforms

			screen.DrawRectShader(screenWidth, int(fontHeight*introFontScale), g.crtShader, &g.drawRectOp)
		} else {
			// Fallback without shader - draw at fixed position
			g.drawOp.GeoM.Reset()
			g.drawOp.ColorScale.Reset()
			g.drawOp.GeoM.Translate(float64(offsetX), float64(yPos))
			screen.DrawImage(g.surfScroll1, &g.drawOp)
		}
		return
	}

	g.drawMainDemo()

	// Final composite with fade - center the original 768-pixel scene inside
	// the wider logical Android surface.
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(float64(offsetX+64), 70)
	op.ColorScale.ScaleAlpha(float32(g.fadeImg))
	screen.DrawImage(g.stCanvas, &op)
}

func logicalWidth(outsideWidth, outsideHeight int) int {
	if outsideWidth <= 0 || outsideHeight <= 0 {
		return screenWidth
	}
	width := (outsideWidth*screenHeight + outsideHeight - 1) / outsideHeight
	if width < screenWidth {
		return screenWidth
	}
	if width > maxLogicalWidth {
		return maxLogicalWidth
	}
	return width
}

// Layout preserves the original scene and uses extra-wide space as black side
// bands instead of stretching the demo on a phone.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return logicalWidth(outsideWidth, outsideHeight), screenHeight
}

// Cleanup releases resources
func (g *Game) Cleanup() {
	if g.audioPlayer != nil {
		if err := g.audioPlayer.Close(); err != nil {
			log.Printf("Failed to close audio player: %v", err)
		}
		g.audioPlayer = nil
	}
	if g.ymPlayer != nil {
		if err := g.ymPlayer.Close(); err != nil {
			log.Printf("Failed to close YM player: %v", err)
		}
		g.ymPlayer = nil
	}
	if g.crtShader != nil {
		g.crtShader.Deallocate()
		g.crtShader = nil
	}
}
