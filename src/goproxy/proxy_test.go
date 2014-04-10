package goproxy

import (
	"testing"
	"net"
	"io"
	"log"
	"bufio"
	"net/http"
	"strconv"
)

import _ "net/http/pprof"

var httpRequest string = "GET http://localhost/ HTTP/1.1\r\n" +
	"Host: localhost\r\n" +
	"User-Agent: Mozilla/5.0 (Macintosh; U; Intel Mac OS X; en-us) AppleWebKit/537+ (KHTML, like Gecko) Version/5.0 Safari/537.6+ Midori/0.4\r\n" +
	"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\n" +
	"Accept-Encoding: gzip, deflate\r\n" +
	"Accept-Language: en\r\n" +
	"Connection: Keep-Alive\r\n\r\n"

var reqLen = len(httpRequest)

var httpReplyBody string = `<html>
<head>
<title>Welcome to the Hello World page</title>
</head>
<body>
<h1>Welcome to the Hello World page</h1>
<p>On this page we are going to say "Hello" to the world.
<hr>
<h2>Hello World!</h2>
<img src="a.jpg" />
<img src="b.jpg" />
<hr>
</body>
</html>
`

var bodLen = len(httpReplyBody)

var httpReply string = "HTTP/1.1 200 OK\r\n" +
	"Vary: Accept-Encoding\r\n" +
	"Last-Modified: Sun, 06 Apr 2014 11:34:10 GMT\r\n" +
	"ETag: \"1381978460\"\r\n" +
	"Content-Type: text/html\r\n" +
	"Accept-Ranges: bytes\r\n" +
	"Content-Length: " + strconv.Itoa(bodLen) + "\r\n" +
	"Date: Sun, 06 Apr 2014 11:43:46 GMT\r\n" +
	"Server: lighttpd/1.4.31\r\n\r\n"

var repLen = len(httpReply)

func TestStartProxy(t *testing.T) {	
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
	
	StartProxy(8080, 8443)
	
	log.Println("Proxy started on port 8080, 8443")
}

func TestParseHeaders(t *testing.T) {
	headerLines := []string { "Host: localhost",
		"Accept-Encoding: gzip, deflate",
		"Accept-Language: en",
		"Connection: Keep-Alive" }

	_, headerVals := parseHeaders(headerLines)

	if (headerVals["Host"] != "localhost") {
		t.Error("parseHeaders didn't work as expected")
	}
}

// Client closes connection after a few requests
func TestCloseConnectionClient(t *testing.T) {
	se, err := net.Listen("tcp", ":80")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("(W) listening on port 80")
	
	closed := make(chan bool)
	go func(se net.Listener, closed chan bool) {
		conn, err := se.Accept()
		if err != nil {
			t.Error(err)
		}
		log.Println("(W) Received connection")
		bw := bufio.NewWriter(conn)
		br := bufio.NewReader(conn)
		brw := bufio.NewReadWriter(br, bw)
		for {
			line, err := brw.ReadString('\n')
			if err == io.EOF {
				log.Println("(W) Connexion closed")
				closed <- true
				return
			}
			if line == "\r\n" {
				log.Println("(W) Received request")
				n, _ := brw.WriteString(httpReply + httpReplyBody)
				brw.Flush()
				log.Println("(W) Sent ", n, " bytes")
			}
			
		}
	}(se, closed)

	connPro, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatal(err)		
	}
	log.Println("(C) Connecting to proxy")
	
	bw := bufio.NewWriter(connPro)
	br := bufio.NewReader(connPro)
	brw := bufio.NewReadWriter(br, bw)

	for i := 0; i < 5; i++ {
		log.Println("(C) Sending request to proxy")
		brw.WriteString(httpRequest)
		brw.Flush()
		log.Println("(C) Receiving reply from proxy")
		_, n, _ := receiveBytes(brw.Reader, repLen + bodLen)
		if (n != repLen + bodLen) {
			t.Error("Received a different size")
		}
	}
	connPro.Close()

	if (true != <-closed) {
		t.Error("Connection not closed on server")
	}

	se.Close()
}

// Server closes connection after a few replies
func TestCloseConnectionServer(t *testing.T) {
	se, err := net.Listen("tcp", ":80")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("(W) listening on port 80")
	
	go func(se net.Listener) {
		conn, err := se.Accept()
		if err != nil {
			t.Error(err)
		}
		log.Println("(W) Received connection")
		bw := bufio.NewWriter(conn)
		br := bufio.NewReader(conn)
		brw := bufio.NewReadWriter(br, bw)
		for i := 0; i < 5; {
			line, err := brw.ReadString('\n')
			if err == io.EOF {
				t.Error("Connection closed by client")
			}
			if line == "\r\n" {
				log.Println("(W) Received request")
				n, _ := brw.WriteString(httpReply + httpReplyBody)
				brw.Flush()
				log.Println("(W) Sent ", n, " bytes")
				i++
			}
		}
		conn.Close()
		log.Println("(W) Connexion closed")
		
	}(se)

	connPro, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatal(err)		
	}
	log.Println("(C) Connecting to proxy")
	
	bw := bufio.NewWriter(connPro)
	br := bufio.NewReader(connPro)
	brw := bufio.NewReadWriter(br, bw)

	for {
		log.Println("(C) Sending request to proxy")
		// _, err := brw.WriteString(httpRequest)
		nn, err := connPro.Write([]byte(httpRequest))
		log.Println("(C) Sent ", nn, " bytes to proxy")
		if err != nil {
			break
		}
		//brw.Flush()
		_, n, _ := receiveBytes(brw.Reader, repLen + bodLen)
		log.Println("(C) Received reply from proxy: ", n, " bytes")
		//connPro.Close()
		//if (n != repLen + bodLen) {
		//	t.Error("Received a different size")
		//}
	}
	se.Close()	
}
