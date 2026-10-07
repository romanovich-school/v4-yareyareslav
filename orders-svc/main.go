package main

import (
	"fmt"
	"log"
	"marketplace/orders-svc/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(cfg)
}
