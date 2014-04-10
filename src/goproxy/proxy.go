package goproxy

import (
	"net"
	"fmt"
	"log"
	"bufio"
	"strings"
	"strconv"
	"io"
	"time"
)
import _ "net/http/pprof"

type connHandle func(connCli net.Conn)

type HttpRequest struct {
	method, args string
	headers []string
	headerVals map[string]string
	body string
}

type HttpReply struct {
	status string
	headers []string
	headerVals map[string]string
	body string
}

func (h *HttpRequest) String() string {
	str := ""
	str += fmt.Sprintf("%s %s\r\n", h.method, h.args)
	for _, header := range h.headers {
		str += fmt.Sprintf("%s: %s\r\n", header, h.headerVals[header])
	}
	str += fmt.Sprintf("\r\n")
	return str
}

func (h *HttpReply) String() string {
	str := ""
	str += fmt.Sprintf("%s\r\n", h.status)
	for _, header := range h.headers {
		str += fmt.Sprintf("%s: %s\r\n", header, h.headerVals[header])
	}
	str += fmt.Sprintf("\r\n")
	return str
}


func StartProxy(httpPort, httpsPort uint) {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Ltime)
	
	ln, err := net.Listen("tcp", fmt.Sprint(":", httpPort))
	if err != nil {
		log.Fatal(err)
	}
	lns, err := net.Listen("tcp", fmt.Sprint(":", httpsPort))
	if err != nil {
		log.Fatal(err)
	}

	go listenLoop(ln, handleHttpCon)
	go listenLoop(lns, handleHttpsCon)
}

func listenLoop(ln net.Listener, h connHandle) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go h(conn)
	}
}

func handleHttpsCon(connCli net.Conn) {
	
}

func handleHttpCon(connCli net.Conn) {
	defer connCli.Close()
	
	writerCli := bufio.NewWriter(connCli)
	readerCli := bufio.NewReader(connCli)
	bufCli := bufio.NewReadWriter(readerCli, writerCli)

	// Need to receive first request to determine connection to server
	req, err := receiveHttpRequest(bufCli.Reader)
	if err != nil {
		log.Println(err)
		return
	}
	//fmt.Println(req)

	connSer, err := getServerConn(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer connSer.Close()

	writerSer := bufio.NewWriter(connSer)
	readerSer := bufio.NewReader(connSer)
	bufSer := bufio.NewReadWriter(readerSer, writerSer)

	for {
		switch req.method {
		case "CONNECT":
			methodConnect(req, bufCli, bufSer)
			return
		case "GET":
			err := methodGet(req, bufCli, bufSer)
			if err != nil {
				return
			}
			connSer.SetReadDeadline(time.Now().Add(time.Millisecond * 40))
			one := make([]byte, 1)
			if _, err := connSer.Read(one); err == io.EOF {
				log.Println("Server closed connection")
				connCli.Close()
				return
			} else {
				connSer.SetReadDeadline(time.Time{})
			}
		case "PUT":
			return
		case "POST":
			return
		default:
			return
		}
		
		req, err = receiveHttpRequest(bufCli.Reader)
		if err != nil {
			log.Println(err)
			return
		}
		//fmt.Println(req)
	}	
}

func getServerConn(req *HttpRequest) (connSer net.Conn, err error) {
	// TODO add dial timeout
	if (req.method == "CONNECT") {
		connSer, err = net.Dial("tcp", strings.Split(req.args, " ")[0])
	} else {
		connSer, err = net.Dial("tcp", req.headerVals["Host"] + ":80")
	}
	if err != nil {
		log.Println(err)		
		return nil, err
	}
	return connSer, nil
}

func methodConnect(req *HttpRequest, bufCli, bufSer *bufio.ReadWriter) {
	bufCli.WriteString("HTTP/1.1 200 OK\r\n\r\n")
	bufCli.Flush()

	tunnel(bufCli, bufSer)
	
	return
}

func methodGet(req *HttpRequest, bufCli, bufSer *bufio.ReadWriter) (err error){
	err = sendHttpRequest(req, bufSer.Writer)
	if err != nil {
		log.Println(err)		
		return err
	}
	bufSer.Flush()
	
	rep, err := receiveHttpReply(bufSer.Reader)
	if err != nil {
		log.Println(err)		
		return err
	}
	
	//fmt.Print("= Request =\n", req, "\n===========\n||\n")
	//fmt.Print("== Reply ==\n", rep, "\n===========\n\n")

	sendHttpReply(rep, bufCli.Writer)
	if err != nil {
		log.Println(err)		
		return err
	}
	
	if rep.headerVals["Transfer-Encoding"] == "chunked" {
		transferChunked(bufSer, bufCli)
	} else {
		length, _ := strconv.Atoi(rep.headerVals["Content-Length"])
		// Receive body from server
		body, n, err := receiveBytes(bufSer.Reader, length)
		if err != nil && n < length {
			log.Println(err)
			return err
		}
		// Send body to client
		_, err = bufCli.Write(body)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	bufCli.Flush()
	return nil
}

func transferChunked(bufSer *bufio.ReadWriter, bufCli *bufio.ReadWriter) (err error) {
	for {
		// Receive chunk length
		line, _ := bufSer.ReadString('\n')
		length, _ := strconv.ParseUint(line[:len(line) - 2], 16, 64)
		// fmt.Println("Reading chunk lenght:", length)
		
		bufCli.WriteString(fmt.Sprintf("%x\r\n", length))
		
		// Receive chunk
		chunk, n, err := receiveBytes(bufSer.Reader, int(length))
		if err != nil && n < int(length) {
			log.Println(err)
			return err
		}
		// Discard \r\n
		receiveBytes(bufSer.Reader, 2)
			
		bufCli.Write(chunk)
		bufCli.WriteString("\r\n")
		
		// Check for end of chunked data
		if length == 0 {
			break
		}
	}
	return err
}

// Tunnel connections and block
func tunnel(rw0 io.ReadWriter, rw1 io.ReadWriter) {	
	go io.Copy(rw0, rw1)
	io.Copy(rw1, rw0)
	return
}

func receiveHttpRequest(reader *bufio.Reader) (req *HttpRequest, err error){
	reqLines, err := receiveHeader(reader)
	if err != nil {		
		return nil, err
	}
	
	req, err = parseHttpRequest(reqLines)
	if err != nil {
		return nil, err
	}
	
	return req, nil
}

func receiveHttpReply(reader *bufio.Reader) (req *HttpReply, err error){
	repLines, err := receiveHeader(reader)
	if err != nil {
		return nil, err
	}
	
	rep, err := parseHttpReply(repLines)
	if err != nil {
		return nil, err
	}
	
	return rep, nil
}
	
func receiveHeader(reader *bufio.Reader) ([]string, error) {
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		// Check end of header
		if line == "\r\n" {
			break
		}
		lines = append(lines, line[:len(line) - 2])
	}
	return lines, nil
}

func receiveBytes(reader *bufio.Reader, length int) (body []byte, nRead int, err error) {
	body = make([]byte, length)

	for nRead < length {
		n, er := reader.Read(body[nRead:])
		nRead += n

		if er != nil {
			err = er
			break
		}
	}
	return body, nRead, err
}

func parseHttpRequest(reqLines []string) (*HttpRequest, error) {
	req := &HttpRequest{}

	r := strings.SplitN(reqLines[0], " ", 2)
	if len(r) < 2 {
		return nil, fmt.Errorf("Bad request: %s", reqLines[0])
	}
	req.method = r[0]
	req.args = r[1]
	
	req.headers, req.headerVals = parseHeaders(reqLines[1:])
	
	return req, nil
}

func sendHttpRequest(req *HttpRequest, writer *bufio.Writer) (err error){
	_, err = writer.WriteString(req.String())
	return err
}

func sendHttpReply(rep *HttpReply, writer *bufio.Writer) (err error) {
	_, err = writer.WriteString(rep.String())
	return err
}

func parseHttpReply(repLines []string) (*HttpReply, error) {
	rep := &HttpReply{}
	
	rep.status = repLines[0]
	rep.headers, rep.headerVals = parseHeaders(repLines[1:])
	
	return rep, nil
}

func parseHeaders(repLines []string) ([]string, map[string]string) {
	headers := []string{}
	headerVals := make(map[string]string)
	
	for _, line := range repLines {
		h := strings.SplitN(line, ": ", 2)
		if len(h) < 2 {
			log.Println("Bad header, ignoring: %s", line)
			continue
		}
		headers = append(headers, h[0])
		headerVals[h[0]] = h[1]
	}
	return headers, headerVals
}
