package main

import (
	"embed"
	"log"

	"gix/internal/app"
	"gix/internal/config"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var trayIcon []byte

func main() {
	config.LoadDotEnv()
	if err := app.Run(assets, trayIcon); err != nil {
		log.Fatal(err)
	}
}
