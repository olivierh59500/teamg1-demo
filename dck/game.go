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
	"github.com/olivierh59500/democonstructionkit/sprites"

	_ "image/png"
	"log"

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

// Game represents the main demo state
type Game struct {
	profiledScroll *scrolling.Scrolling
	// Images
	fontImg     *ebiten.Image
	teamG1Logo  *ebiten.Image
	gameOneLogo *ebiten.Image
	texture     *ebiten.Image

	// Canvases
	stCanvas   *ebiten.Image
	cubeCanvas *ebiten.Image
	logoCanvas *ebiten.Image

	// Effects
	plasmaField *plasma.HarmonicImage
	logoWave    *composite.ProfileImage

	// Shared live-texture cube.
	cube *effects.TexturedCube

	// Independently phased sprite formation.
	logoFormation *sprites.Group

	// Intro scrolling

	// Animation state
	fadeImg       float64
	introComplete bool

	// Audio
	audioContext *audio.Context
	audioPlayer  *audio.Player
	musicStream  *sound.Stream
	audioReady   bool
	musicStarted bool

	// Animated intro material.
	crt *effects.TimedCRTOverlay

	// Font data
	fontAtlas *scrolling.Atlas

	// Finite intro text transport.
	introScroll *scrolling.Scrolling

	// Draw options (optimization)
	drawOp ebiten.DrawImageOptions
}

// NewGame creates and initializes a new game instance
func NewGame() *Game {
	g := &Game{
		fadeImg: 2.0,
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
	g.cubeCanvas = ebiten.NewImage(stCanvasWidth, stCanvasHeight)
	g.logoCanvas = ebiten.NewImage(stCanvasWidth, stCanvasHeight)

	// Initialize font data
	var err error
	g.fontAtlas, err = presets.FontAtlas("teamg1-demo", g.fontImg)
	if err != nil {
		panic(err)
	}
	introConfig := presets.TeamG1IntroFeed(g.fontAtlas, introScrollText)
	intro, err := scrolling.New(scrolling.Config{Feed: &introConfig})
	if err != nil {
		panic(err)
	}
	g.introScroll = intro
	profiledConfig := presets.TeamG1ProfiledScroll(g.fontAtlas, scrollText)
	g.profiledScroll, err = scrolling.New(scrolling.Config{Profiled: &profiledConfig})
	if err != nil {
		panic(err)
	}

	// Bind the live source texture to the complete cube effect.
	g.cube, err = effects.NewTexturedCube(g.texture, presets.TeamG1TexturedCube())
	if err != nil {
		panic(err)
	}

	// Configure the reusable sprite/logo formation.
	g.logoFormation, err = sprites.NewGroup(presets.TeamG1LogoFormation(
		g.gameOneLogo, stCanvasWidth, stCanvasHeight))
	if err != nil {
		panic(err)
	}

	// The effect owns its pixels, live GPU surface and dirty-frame upload.
	g.plasmaField, err = plasma.NewHarmonicImage(
		plasma.DefaultHarmonicConfig(stCanvasWidth/2, stCanvasHeight/2), plasmaSpeed)
	if err != nil {
		panic(err)
	}

	// Configure the complete, reusable row-profile logo effect.
	g.logoWave, err = composite.NewProfileImage(g.teamG1Logo, presets.TeamG1LogoProfile(float64(stCanvasWidth)))
	if err != nil {
		panic(err)
	}

	// Compile CRT shader
	g.crt, err = effects.NewTimedCRTOverlay(presets.TeamG1TimedCRT())
	if err != nil {
		log.Printf("Failed to compile CRT shader: %v", err)
	}

	return g
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

// drawMainDemo draws the main demo scene
func (g *Game) drawMainDemo() {
	g.stCanvas.Fill(color.Black)
	// Draw the live plasma surface at the authored 2x scale.
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(2, 2)
	if image := g.plasmaField.Image(); image != nil {
		g.stCanvas.DrawImage(image, &op)
	}

	// Draw textured cube
	g.cubeCanvas.Clear()
	g.cube.Draw(g.cubeCanvas)
	op = ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(0.8)
	g.stCanvas.DrawImage(g.cubeCanvas, &op)

	// Draw distorted TEAMG1 logo
	g.logoWave.Draw(g.stCanvas)

	// Draw scrolling text
	g.profiledScroll.Draw(g.stCanvas)

	// Draw logo spiral
	g.logoCanvas.Clear()
	g.logoFormation.Draw(g.logoCanvas)
	op = ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(0.6)
	g.stCanvas.DrawImage(g.logoCanvas, &op)
}

func (g *Game) advanceMainDemo() error {
	if err := g.plasmaField.Update(kit.Frame{}); err != nil {
		return err
	}
	g.cube.Rotate(.02, .03, .01)
	if err := g.logoFormation.Update(kit.Frame{}); err != nil {
		return err
	}
	g.logoWave.Advance()

	if err := g.profiledScroll.Update(kit.Frame{}); err != nil {
		return err
	}
	return nil
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
		} else if g.crt != nil {
			if err := g.crt.Update(kit.Frame{}); err != nil {
				return err
			}
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

		if err := g.advanceMainDemo(); err != nil {
			return err
		}
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

		if g.crt != nil {
			g.crt.DrawAt(screen, g.introScroll.Image(), float64(offsetX), float64(yPos))
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
	if g.profiledScroll != nil {
		g.profiledScroll.Close()
	}
	if g.plasmaField != nil {
		g.plasmaField.Close()
	}
	if g.logoWave != nil {
		g.logoWave.Close()
	}
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
	if g.crt != nil {
		g.crt.Close()
	}
}
