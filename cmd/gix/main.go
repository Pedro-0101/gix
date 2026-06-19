package main

import (
	"gix/internal/config"
	"gix/internal/ui"
)

func main() {
	config.LoadDotEnv()
	ui.Run()
}
