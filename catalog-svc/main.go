package main

import (
	"fmt"
	"marketplace/catalog-svc/config"
)

func run() error {
	_, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	return nil
}

func main() {
	run()
}
