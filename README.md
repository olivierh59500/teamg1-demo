# TEAMG1 Demo

A modern tribute to the golden age of demoscene, created with Go and Ebiten by Bilizir from DMA.

## Overview

This demo combines classic demoscene effects from the late 80s/early 90s with modern twists:

- **CRT Shader Intro**: Enhanced CRT effect with phosphor glow, scanlines, and barrel distortion
- **Plasma Background**: Real-time generated plasma effect using multiple sine waves
- **3D Textured Cube**: Fully textured rotating cube with proper backface culling
- **Logo Deformation**: TEAMG1 logo with sinusoidal distortion
- **Spiral Logos**: Multiple GAMEONE logos rotating in a spiral pattern
- **Wave Scroller**: Classic scrolling text with wave distortion
- **YM Chiptune Music**: Authentic Atari ST sound using YM2149 emulation

## Features

### Visual Effects
- Enhanced CRT shader with multiple effects (scanlines, RGB shift, vignette, flicker)
- Real-time plasma field generation
- 3D textured cube with perspective-correct rendering
- Logo deformation and animation
- Multiple scrolling text layers with different effects
- Smooth transitions between scenes

### Technical Features
- Optimized for cross-platform performance (Windows, macOS, Linux)
- Support for both Intel and ARM architectures
- Pre-rendered frames for smooth animations
- Efficient memory usage with canvas reuse
- Hardware-accelerated rendering via Ebiten

### Audio
- YM2149 sound chip emulation for authentic chiptune music
- Looped playback with volume control
- Perfect synchronization with visual effects

### Build Instructions

```bash
# Clone the repository
git clone https://github.com/olivierh59500/teamg1-demo
cd teamg1-demo

# Get dependencies
go mod init teamg1-demo
go get github.com/hajimehoshi/ebiten/v2
go get github.com/olivierh59500/ym-player/pkg/stsound

# Build
go build -o teamg1-demo main.go

# Run
./teamg1-demo