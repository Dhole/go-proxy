package main

import (
	"net"
	"fmt"
	"log"
	"bufio"
	"strings"
	"strconv"
)

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

func (h *HttpRequest) Init() {
	h.headerVals = make(map[string]string)
}

func (h *HttpReply) Init() {
	h.headerVals = make(map[string]string)
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
	
	reqLines, err := receiveHeader(readerCli)
	if err != nil {
		log.Println(err)		
		return
	}
	
	req, err := parseHttpRequest(reqLines)
	if err != nil {
		log.Println(err)
		return
	}
	
	// connect to destination server	
	connSer, err := net.Dial("tcp", req.headerVals["Host"] + ":80")
	if err != nil {
		log.Println(err)		
		return
	}
	defer connSer.Close()
	
	readerSer := bufio.NewReader(connSer)
	writerSer := bufio.NewWriter(connSer)

	sendHttpRequest(req, writerSer)
	writerSer.Flush()
	
	repLines, err := receiveHeader(readerSer)
	if err != nil {
		log.Println(err)		
		return
	}
	
	rep, err := parseHttpReply(repLines)
	if err != nil {
		log.Println(err)		
		return
	}
	
	fmt.Println("= Request =\n", req.String(), "\n===========\n||")
	fmt.Println("== Reply ==\n", rep.String(), "\n===========\n")

	sendHttpReply(rep, writerCli)

	length, _ := strconv.Atoi(rep.headerVals["Content-Length"])
	body, err := receiveBody(readerSer, length)
	if err != nil {
		log.Println(err)
		return
	}
	
	_, err = writerCli.Write(body)
	if err != nil {
		log.Println(err)
		return
	}

	writerCli.Flush()
	
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

func receiveBody(reader *bufio.Reader, length int) ([]byte, error) {
	
	body := make([]byte, length)
	nRead := 0

	for nRead < length {
		n, err := reader.Read(body[nRead:])
		if err != nil {
			return nil, err
		}
		nRead += n
	}
	return body, nil
}

func parseHttpRequest(reqLines []string) (*HttpRequest, error) {
	req := HttpRequest{}
	req.Init()
	isFirstLine := true
	
	for _, line := range reqLines {
		if isFirstLine {
			isFirstLine = false
			
			r := strings.SplitN(line, " ", 2)
			if len(r) < 2 {
				return nil, fmt.Errorf("Bad request: %s", line)
			}
			req.method = r[0]
			req.args = r[1]
		} else {
			h := strings.SplitN(line, ": ", 2)
			if len(h) < 2 {
				log.Println("Bad header, ignoring: %s", line)
				continue
			}
			req.headers = append(req.headers, h[0])
			req.headerVals[h[0]] = h[1]
		}
	}
	return &req, nil
}

func sendHttpRequest(req *HttpRequest, writer *bufio.Writer) {
	writer.WriteString(req.String())
}

func sendHttpReply(rep *HttpReply, writer *bufio.Writer) {
	writer.WriteString(rep.String())
}

func parseHttpReply(repLines []string) (*HttpReply, error) {
	rep := HttpReply{}
	rep.Init()
	isFirstLine := true
	
	for _, line := range repLines {
		if isFirstLine {
			isFirstLine = false
			rep.status = line
		} else {
			h := strings.SplitN(line, ": ", 2)
			if len(h) < 2 {
				log.Println("Bad header, ignoring: %s", line)
				continue
			}
			rep.headers = append(rep.headers, h[0])
			rep.headerVals[h[0]] = h[1]
		}
	}
	return &rep, nil
}
