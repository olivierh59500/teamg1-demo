// Package mobile exposes the TEAMG1 demo to ebitenmobile.
package mobile

import (
	enginemobile "github.com/hajimehoshi/ebiten/v2/mobile"

	teamg1demo "teamg1-demo"
)

func init() {
	enginemobile.SetGame(teamg1demo.NewGame())
}

// Dummy forces gomobile to include this package in the Android binding.
func Dummy() {}
