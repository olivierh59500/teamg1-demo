package teamg1demo

import (
	"bytes"
	"math"
	"testing"
)

// This retained renderer is the pre-extraction implementation. It validates
// actual output, including channel rounding, against the shared kernel.
type legacyPlasmaField struct {
	time        float64
	width       int
	height      int
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

func newLegacyPlasmaFieldForSize(width, height int) *legacyPlasmaField {
	pixelCount := width * height
	diagonalCount := width + height - 1
	p := &legacyPlasmaField{
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

func legacyPlasmaColor(value float64) byte {
	value = (value + 1) * 127
	if value <= 0 {
		return 0
	}
	if value >= 254 {
		return 254
	}
	return byte(value)
}

func (p *legacyPlasmaField) updatePixels() {
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
			p.pixels[pixel] = legacyPlasmaColor(sinColor)
			p.pixels[pixel+1] = legacyPlasmaColor(-0.5*sinColor + sinTwoPiThird*cosColor)
			p.pixels[pixel+2] = legacyPlasmaColor(-0.5*sinColor - sinTwoPiThird*cosColor)
			p.pixels[pixel+3] = 0xff
		}
	}
}

func TestSharedHarmonicPreservesEveryPixel(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {32, 20}, {320, 200}, {83, 47}} {
		old := newLegacyPlasmaFieldForSize(size[0], size[1])
		shared := newPlasmaFieldForSize(size[0], size[1])
		for _, seconds := range []float64{0, .02, .42, 7.5, -3, 9999} {
			old.time, shared.time = seconds, seconds
			old.updatePixels()
			shared.updatePixels()
			if !bytes.Equal(old.pixels, shared.pixels) {
				for i := range old.pixels {
					if old.pixels[i] != shared.pixels[i] {
						t.Fatalf("size %v time %g byte %d: shared %d, original %d", size, seconds, i, shared.pixels[i], old.pixels[i])
					}
				}
			}
		}
	}
}

func BenchmarkSharedAndOriginalHarmonic(b *testing.B) {
	b.Run("shared", func(b *testing.B) {
		p := newPlasmaFieldForSize(320, 200)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			p.time += .02
			p.updatePixels()
		}
	})
	b.Run("original", func(b *testing.B) {
		p := newLegacyPlasmaFieldForSize(320, 200)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			p.time += .02
			p.updatePixels()
		}
	})
}
