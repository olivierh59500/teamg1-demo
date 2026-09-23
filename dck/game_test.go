package teamg1demo

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/olivierh59500/democonstructionkit/sound"
)

func TestMusicStreamReadProducesStereoWithoutAllocating(t *testing.T) {
	player, err := sound.Open("music.ym", musicData, sound.Options{SampleRate: sampleRate, Loop: true, PCMFormat: sound.PCM16, Gain: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := player.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	buffer := make([]byte, 4096*4)
	read := func() {
		n, err := player.Read(buffer)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if n != len(buffer) {
			t.Fatalf("Read bytes = %d, want %d", n, len(buffer))
		}
	}

	read()
	for i := 0; i < len(buffer); i += 4 {
		left := binary.LittleEndian.Uint16(buffer[i : i+2])
		right := binary.LittleEndian.Uint16(buffer[i+2 : i+4])
		if left != right {
			t.Fatalf("frame %d is not mono duplicated to stereo: %d != %d", i/4, left, right)
		}
	}

	if allocations := testing.AllocsPerRun(20, read); allocations != 0 {
		t.Fatalf("Read allocations = %v, want 0", allocations)
	}
}

func TestLogicalWidth(t *testing.T) {
	tests := []struct {
		name          string
		outsideWidth  int
		outsideHeight int
		want          int
	}{
		{name: "invalid", outsideWidth: 0, outsideHeight: 0, want: screenWidth},
		{name: "original", outsideWidth: 768, outsideHeight: 540, want: 768},
		{name: "pixel landscape", outsideWidth: 2424, outsideHeight: 1080, want: 1212},
		{name: "portrait clamps to scene", outsideWidth: 1080, outsideHeight: 2424, want: 768},
		{name: "ultrawide cap", outsideWidth: 4000, outsideHeight: 1000, want: maxLogicalWidth},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := logicalWidth(test.outsideWidth, test.outsideHeight); got != test.want {
				t.Fatalf("logicalWidth(%d, %d) = %d, want %d", test.outsideWidth, test.outsideHeight, got, test.want)
			}
		})
	}
}

func TestPlasmaUpdateMatchesOriginalFormulaWithoutAllocating(t *testing.T) {
	p := newPlasmaFieldForSize(32, 20)
	p.time = 0.42
	p.updatePixels()

	points := [][2]int{{0, 0}, {7, 3}, {31, 19}, {16, 10}}
	for _, point := range points {
		x, y := point[0], point[1]
		v1 := math.Sin(float64(x)*0.02 + p.time)
		v2 := math.Sin(float64(y)*0.03 + p.time*1.5)
		v3 := math.Sin(math.Sqrt(float64(x*x+y*y))*0.01 + p.time*0.5)
		v4 := math.Sin((float64(x)+float64(y))*0.01 + p.time*2)
		value := (v1 + v2 + v3 + v4) / 4
		want := [3]byte{
			byte((math.Sin(value*math.Pi) + 1) * 127),
			byte((math.Sin(value*math.Pi+2*math.Pi/3) + 1) * 127),
			byte((math.Sin(value*math.Pi+4*math.Pi/3) + 1) * 127),
		}
		pixel := (y*p.width + x) * 4
		got := p.pixels[pixel : pixel+4]
		for channel := range 3 {
			if delta := math.Abs(float64(got[channel]) - float64(want[channel])); delta > 1 {
				t.Fatalf("pixel (%d,%d), channel %d = %d, want %d", x, y, channel, got[channel], want[channel])
			}
		}
		if got[3] != 0xff {
			t.Fatalf("pixel (%d,%d) alpha = %d, want 255", x, y, got[3])
		}
	}

	if allocations := testing.AllocsPerRun(20, p.updatePixels); allocations != 0 {
		t.Fatalf("plasma update allocations = %v, want 0", allocations)
	}
}

func BenchmarkPlasmaUpdatePixels(b *testing.B) {
	p := newPlasmaFieldForSize(stCanvasWidth/2, stCanvasHeight/2)
	b.ReportAllocs()
	for b.Loop() {
		p.advance()
		p.updatePixels()
	}
}

func BenchmarkMusicStreamRead4096(b *testing.B) {
	player, err := sound.Open("music.ym", musicData, sound.Options{SampleRate: sampleRate, Loop: true, PCMFormat: sound.PCM16, Gain: 0.5})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := player.Close(); err != nil {
			b.Errorf("Close: %v", err)
		}
	})

	buffer := make([]byte, 4096*4)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := player.Read(buffer); err != nil {
			b.Fatal(err)
		}
	}
}
