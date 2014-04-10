package main

import (
	"goproxy"
	"net/http"
)

func main() {
	// enable profiling on http://localhost:6060/debug/pprof
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	goproxy.StartProxy(8080, 8443)

	c := make(chan int)
	<-c
}
