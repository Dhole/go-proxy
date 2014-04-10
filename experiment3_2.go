package main

import (
	"log"
	"time"
)

func main() {
	a()
	time.Sleep(1000000000)
}

func a() {
	log.Println("Begin a")
	go func() {
		log.Println("Begin anon")
		time.Sleep(100)
		log.Println("End anon")
	}()
	log.Println("End a")
}
