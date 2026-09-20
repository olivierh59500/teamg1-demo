package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	teamg1demo "teamg1-demo/dck"
)

func run() error {
	ebiten.SetWindowSize(768, 540)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("TEAMG1 Demo - A Tribute to the Golden Age")
	ebiten.SetScreenClearedEveryFrame(false)

	game := teamg1demo.NewGame()
	defer game.Cleanup()
	return ebiten.RunGame(newDrawOnUpdateGame(game))
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
