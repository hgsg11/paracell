package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hgsg11/paracell/internal/app"
)

func main() {
	workdir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := app.Run(context.Background(), os.Args[1:], workdir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
