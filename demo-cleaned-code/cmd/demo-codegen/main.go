package main

import (
	"fmt"
	"os"

	"github.com/strengthinnumbers-business/client-reminder/internal/democlean"
)

func main() {
	if err := democlean.Generate(democlean.Options{
		SourceDir: ".",
		OutputDir: "demo-cleaned-code",
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
