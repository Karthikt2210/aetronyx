package main

import (
	"os"

	"github.com/karthikcodes/aetronyx/cmd/aetronyx"
)

func main() {
	if err := aetronyx.Execute(); err != nil {
		os.Exit(1)
	}
}
