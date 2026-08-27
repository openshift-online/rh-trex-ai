package main

import (
	"flag"
	"fmt"
	"log"
)

func main() {
	options := generateOptions{}
	flag.StringVar(&options.SpecPath, "spec", "", "path to the root OpenAPI document")
	flag.StringVar(&options.OutDir, "out", "", "generated in-module TUI descriptor package")
	flag.Parse()
	if err := generate(options); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Embedded TUI descriptor updated in %s\n", options.OutDir)
}
