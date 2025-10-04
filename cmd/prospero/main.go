package main

import (
	"github.com/joho/godotenv"
	"prospero/internal/app/cli"
)

func main() {
	_ = godotenv.Load()
	cli.Execute()
}
