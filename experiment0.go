package main

import (
	"log"
)

func main() {
	go func(msg string) {
		log.Fatal(msg)
	}("Bye")
	for { }
}
