package main

import (
	"log"
	"time"

	"deepstateclicker/internal/game"
	"deepstateclicker/internal/terminal"
)

func main() {
	app := terminal.New(game.NewState(time.Now()))
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
