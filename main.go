package main

import (
	"github.com/Jan-Kur/HackCLI/cmd"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		panic("Couldn't load .env")
	}

	cmd.Execute()
}
