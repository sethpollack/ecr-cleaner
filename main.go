package main

import (

	// agent "github.com/ltvco/ltv-apm-modules-go/agent"

	"os"
	"strconv"

	"github.com/ltvco/ecr-cleaner/cmd"
	"github.com/rs/zerolog"
)

func main() {
	logLevel, err := strconv.Atoi(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = int(zerolog.InfoLevel) // default to INFO
	}
	zerolog.SetGlobalLevel(zerolog.Level(logLevel))

	cmd.Execute()
}
