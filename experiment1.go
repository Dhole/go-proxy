package main

import (
	"log"
	"net"
	"bufio"
)

func main() {
	ln, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleHttpCon(conn)
	}
}

func handleHttpCon(connCli net.Conn) {
	defer connCli.Close()
	
	readerCli := bufio.NewReader(connCli)

	buf := make([]byte, 1500 * 2)

	for {
		nr, err := readerCli.Read(buf)
		if nr > 0 {
			log.Println("Read:", nr, "bytes", "Error:", err)
			log.Println(buf[0:nr])
		}
		if err != nil {
			log.Println(err)
			return
		}
	}
}
