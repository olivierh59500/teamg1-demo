// Package teamg1demo implements the TEAMG1 demoscene tribute.
package teamg1demo

import (
	"bytes"
	"github.com/olivierh59500/democonstructionkit/presets"
	"image"
	"image/color"
	originalassets "teamg1-demo"

	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/plasma"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/democonstructionkit/sound"

	_ "image/png"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	audio "github.com/olivierh59500/democonstructionkit/sound/output"
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
	fontData = originalassets.DCKAssetFontData()

	teamG1LogoData = originalassets.DCKAssetTeamG1LogoData()

	gameOneLogoData = originalassets.DCKAssetGameOneLogoData()

	textureData = originalassets.DCKAssetTextureData()

	musicData = originalassets.

		// Letter represents a character in the bitmap font
		DCKAssetMusicData()
)

// PlasmaField represents the plasma effect background
type PlasmaField struct {
	time          float64
	width, height int
	buffer        *ebiten.Image
	pixels        []byte
	kernel        *plasma.Harmonic
	dirty         bool
}

// LogoDistortion handles the logo distortion effect
type LogoDistortion struct {
	distSin   []float64
	distCount int
}

type scrollGlyph struct {
	image *ebiten.Image
	width float64
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

	// Shared live-texture cube.
	cube *effects.TexturedCube

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

	// Animation state
	fadeImg       float64
	shaderTime    float64
	introComplete bool

	// Audio
	audioContext *audio.Context
	audioPlayer  *audio.Player
	musicStream  *sound.Stream
	audioReady   bool
	musicStarted bool

	// Shader
	crtShader   *ebiten.Shader
	crtUniforms map[string]any

	// Font data
	fontAtlas       *scrolling.Atlas
	teamG1LogoLines []*ebiten.Image

	// Finite intro text transport.
	introScroll *scrolling.Scrolling

	// Draw options (optimization)
	drawOp     ebiten.DrawImageOptions
	drawRectOp ebiten.DrawRectShaderOptions
}

// NewGame creates and initializes a new game instance
func NewGame() *Game {
	g := &Game{
		fadeImg:        2.0,
		crtUniforms:    map[string]any{"Time": float32(0)},
		scrollVertices: make([]ebiten.Vertex, 0, int(fontHeight*demoFontScale/2)*4),
		scrollIndices:  make([]uint16, 0, int(fontHeight*demoFontScale/2)*6),
	}

	// Initialize scrolling texts
	spc := "     "
	introScrollText := spc +
		"C'EST MERCREDI..." + spc +
		"JE REPETE, C'EST MERCREDI ET LE MERCREDI..." + spc

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

	// Initialize font data
	g.initFontData()
	introConfig := presets.TeamG1IntroFeed(g.fontAtlas, introScrollText)
	intro, err := scrolling.New(scrolling.Config{Feed: &introConfig})
	if err != nil {
		panic(err)
	}
	g.introScroll = intro
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
	var err error
	g.fontAtlas, err = presets.FontAtlas("teamg1-demo", g.fontImg)
	if err != nil {
		panic(err)
	}
}

func (g *Game) initScrollText(text []rune) {
	g.scrollGlyphs = make([]scrollGlyph, 0, len(text))
	for _, char := range text {
		glyphImage, letter, ok := g.fontAtlas.ExactGlyph(char)
		if !ok {
			width := 32 * demoFontScale
			g.scrollGlyphs = append(g.scrollGlyphs, scrollGlyph{width: width})
			g.scrollTextWidth += width
			continue
		}

		width := float64(int(letter.Advance)) * demoFontScale
		g.scrollGlyphs = append(g.scrollGlyphs, scrollGlyph{
			image: glyphImage,
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
	kernel, err := plasma.NewHarmonic(plasma.DefaultHarmonicConfig(width, height))
	if err != nil {
		panic(err)
	}
	return &PlasmaField{width: width, height: height, pixels: make([]byte, width*height*4), kernel: kernel, dirty: true}
}

func (p *PlasmaField) advance() {
	p.time += plasmaSpeed
	p.dirty = true
}

func (p *PlasmaField) updatePixels() {
	if err := p.kernel.RenderRGBA(p.pixels, p.width*4, p.time); err != nil {
		panic(err)
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
	var err error
	g.cube, err = effects.NewTexturedCube(g.texture, presets.TeamG1TexturedCube())
	if err != nil {
		panic(err)
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

// initAudio opens the soundtrack and starts audio output.
func (g *Game) initAudio() {
	g.audioContext = audio.NewContext(sampleRate)

	musicStream, err := sound.Open("music.ym", musicData, sound.Options{SampleRate: sampleRate, Loop: true, PCMFormat: sound.PCM16, Gain: 0.5})
	if err != nil {
		log.Printf("Failed to open music: %v", err)
		return
	}
	g.musicStream = musicStream

	audioPlayer, err := g.audioContext.NewPlayer(musicStream)
	if err != nil {
		log.Printf("Failed to create audio player: %v", err)
		if closeErr := musicStream.Close(); closeErr != nil {
			log.Printf("Failed to close music stream: %v", closeErr)
		}
		g.musicStream = nil
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

// drawTexturedCube draws the 3D textured cube
func (g *Game) drawTexturedCube() {
	g.cubeCanvas.Clear()
	g.cube.Draw(g.cubeCanvas)
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
	g.cube.Rotate(.02, .03, .01)
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
		if err := g.introScroll.Update(kit.Frame{}); err != nil {
			return err
		}
		if g.introScroll.Finished() {
			g.introComplete = true
			g.fadeImg = 0
		} else {
			g.shaderTime += .016
		}
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
			g.drawRectOp.Images[0] = g.introScroll.Image()
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
			screen.DrawImage(g.introScroll.Image(), &g.drawOp)
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
	if g.introScroll != nil {
		g.introScroll.Close()
	}
	if g.cube != nil {
		g.cube.Close()
	}
	if g.audioPlayer != nil {
		if err := g.audioPlayer.Close(); err != nil {
			log.Printf("Failed to close audio player: %v", err)
		}
		g.audioPlayer = nil
	}
	if g.musicStream != nil {
		if err := g.musicStream.Close(); err != nil {
			log.Printf("Failed to close music stream: %v", err)
		}
		g.musicStream = nil
	}
	if g.crtShader != nil {
		g.crtShader.Deallocate()
		g.crtShader = nil
	}
}
