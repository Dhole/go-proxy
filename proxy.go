package main

import (
	"net"
	"fmt"
	"log"
	"bufio"
	"strings"
	"strconv"
	"io"
	"net/http"
	//	"time"
)
import _ "net/http/pprof"

type HttpRequest struct {
	method, args string
	headers []string
	headerVals map[string]string
}

type HttpReply struct {
	status string
	headers []string
	headerVals map[string]string
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

func main() {

	// enable profiling on http://localhost:6060/debug/pprof
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	
	ln, err := net.Listen("tcp", ":8080")
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
	
	writerCli := bufio.NewWriter(connCli)
	readerCli := bufio.NewReader(connCli)
	bufCli := bufio.NewReadWriter(readerCli, writerCli)

	for {
		req, err := receiveHttpRequest(bufCli.Reader)
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Println(req)
		
		switch req.method {
		case "CONNECT":
			methodConnect(req, bufCli)
			return
		case "GET":
			methodGet(req, bufCli)
		}
		return
	}	
}

func methodConnect(req *HttpRequest, bufCli *bufio.ReadWriter) {
		
	//fmt.Println(req.method, req.args)
	
	connSer, err := net.Dial("tcp", strings.Split(req.args, " ")[0])
	if err != nil {
		log.Println(err)		
		return
	}
	defer connSer.Close()
	
	readerSer := bufio.NewReader(connSer)
	writerSer := bufio.NewWriter(connSer)
	bufSer := bufio.NewReadWriter(readerSer, writerSer)		
	
	bufCli.WriteString("HTTP/1.1 200 OK\r\n\r\n")
	bufCli.Flush()

	tunnel(bufCli, bufSer)
	
	return
}

func methodGet(req *HttpRequest, bufCli *bufio.ReadWriter) {
	// connect to destination server
	// TODO: change Dial for DialTimeout, handle timeout
	connSer, err := net.Dial("tcp", req.headerVals["Host"] + ":80")
	if err != nil {
		log.Println(err)		
		return
	}
	defer connSer.Close()
	
	readerSer := bufio.NewReader(connSer)
	writerSer := bufio.NewWriter(connSer)
	bufSer := bufio.NewReadWriter(readerSer, writerSer)

	sendHttpRequest(req, bufSer.Writer)
	bufSer.Flush()
	
	repLines, err := receiveHeader(bufSer.Reader)
	if err != nil {
		log.Println(err)		
		return
	}
	
	rep, err := parseHttpReply(repLines)
	if err != nil {
		log.Println(err)		
		return
	}
	
	//fmt.Print("= Request =\n", req, "\n===========\n||\n")
	//fmt.Print("== Reply ==\n", rep, "\n===========\n\n")

	sendHttpReply(rep, bufCli.Writer)

	if rep.headerVals["Transfer-Encoding"] == "chunked" {
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
				return
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
	} else {
		length, _ := strconv.Atoi(rep.headerVals["Content-Length"])
		// Receive body from server
		body, n, err := receiveBytes(bufSer.Reader, length)
		if err != nil && n < length {
			log.Println(err)
			return
		}
		// Send body to client
		_, err = bufCli.Write(body)
		if err != nil {
			log.Println(err)
			return
		}
	}
	bufCli.Flush()
}

// Tunnel connections and block
func tunnel(rw0 io.ReadWriter, rw1 io.ReadWriter) {
	
	go io.Copy(rw0, rw1)
	io.Copy(rw1, rw0)
}

func receiveHttpRequest(reader *bufio.Reader) (req *HttpRequest, err error){

	reqLines, err := receiveHeader(reader)
	if err != nil {
		log.Println(err)		
		return nil, err
	}
	
	req, err = parseHttpRequest(reqLines)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	
	return req, nil
}

func receiveHttpReply(reader *bufio.Reader) (req *HttpReply, err error){

	repLines, err := receiveHeader(reader)
	if err != nil {
		log.Println(err)		
		return nil, err
	}
	
	rep, err := parseHttpReply(repLines)
	if err != nil {
		log.Println(err)
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

func sendHttpRequest(req *HttpRequest, writer *bufio.Writer) {
	writer.WriteString(req.String())
}

func sendHttpReply(rep *HttpReply, writer *bufio.Writer) {
	writer.WriteString(rep.String())
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
