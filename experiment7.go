func handleHttpCon(connCli net.Conn) {
	disCli := make(chan bool)
	disSer := make(chan bool)
	reqReady := make(chan bool)
	repReady := make(chan bool)
	req := &HttpRequest{}
	rep := &HttpReply{}

	go readCli()
	go readSer()
	
	for {
		// Request
		select {
		case <- reqReady:
			sendReq(req, bufSer)
		case <- disSer:
			connCli.Close()
			return
		case <- disCli:
			connSer.Close()
			return
		}

		// Reply
		select {
		case <- repReady:
			sendRep(rep, bufCli)
		case <- disSer:
			connCli.Close()
			return
		case <- disCli:
			connSer.Close()
			return
		}
	}
}

func readCli(req *HttpRequest, br bufio.Reader, d chan bool) {
	for {
		req, err := receiveHttpRequest(br)
		if err == io.EOF {
			//log.Println(err)
			log.Println("Client disconnected")
			d <- true
			return
		}
		reqReady <- true
	}
}

func readSer(rep *HttpReply, br bufio.Reader, d chan bool) {
	rep, err := receiveHttpReply(br)
	if err == io.EOF {
		//log.Println(err)
		log.Println("Server disconnected")
		d <- true
		return
	}
	repReady <- true
}
