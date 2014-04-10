package main

import (
	"log"
	"net"
	"bufio"
	"time"
)

func main() {
	conn, err := net.Dial("tcp", "www.upc.edu:80")
        if err != nil {
                log.Fatal(err)          
        }
        
        bw := bufio.NewWriter(conn)
        br := bufio.NewReader(conn)
        // brw := bufio.NewReadWriter(br, bw)

	bw.WriteString("GET / HTTP/1.1\r\n")
	go func(conn net.Conn) {
		time.Sleep(1000 * time.Millisecond)
		conn.Close()
	}(conn)
	log.Println("ReadString")
	line, err := br.ReadString('\n')
	if err != nil {
		log.Println("read: ", line)
		log.Println(err)
	}
}
