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
		defer conn.Close()
		go handleHttpCon(conn)
	}
}

func handleHttpCon(connCli net.Conn) {
	defer connCli.Close()
	
	writerCli := bufio.NewWriter(connCli)
	readerCli := bufio.NewReader(connCli)
	bufCli := bufio.NewReadWriter(readerCli, writerCli)
	
	reqLines, err := receiveHeader(bufCli.Reader)
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
	
	fmt.Println("= Request =\n", req.String(), "\n===========\n||")
	fmt.Println("== Reply ==\n", rep.String(), "\n===========\n")

	sendHttpReply(rep, writerCli)

	if rep.headerVals["Transfer-Encoding"] == "chunked" {
		for {
			// Receive chunk length
			line, _ := bufSer.ReadString('\n')
			length, _ := strconv.ParseUint(line[:len(line) - 2], 16, 64)
			// fmt.Println("Reading chunk lenght:", length)

			bufCli.WriteString(fmt.Sprintf("%x\r\n", length))

			// Receive chunk
			chunk, err := receiveBytes(bufSer.Reader, int(length))
			if err != nil {
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
		body, err := receiveBytes(bufSer.Reader, length)
		if err != nil {
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

func receiveBytes(reader *bufio.Reader, length int) ([]byte, error) {
	
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
