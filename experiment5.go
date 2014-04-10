package main

import (
	"log"
	"net"
	"bufio"
)

var httpRequest string = "GET / HTTP/1.1\r\n\r\n"

func main() {
	conn, err := net.Dial("tcp", "www.upc.edu:80")
        if err != nil {
                log.Fatal(err)          
        }
        
        bw := bufio.NewWriter(conn)
        br := bufio.NewReader(conn)
        brw := bufio.NewReadWriter(br, bw)

        for {
                log.Println("(C) Sending request to proxy")
                // _, err := brw.WriteString(httpRequest)
                nn, err := conn.Write([]byte(httpRequest))
                log.Println("(C) Sent ", nn, " bytes to proxy")
                if err != nil {
                        break
                }
                //brw.Flush()
                n, _ := brw.ReadString('\n')
                log.Println("(C) Received reply from proxy: ", n, " bytes")
        }
}

