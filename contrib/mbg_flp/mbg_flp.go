package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/hpcloud/tail"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
)

type clientInfo struct {
	address string
}

var (
	activeConnections map[string]api.MbgConnectionInfo
	activeClients     map[string]net.Conn
)

func main() {
	logFileName := os.Args[1]
	fmt.Printf("logfilename = %s \n", logFileName)
	activeConnections = make(map[string]api.MbgConnectionInfo)
	activeClients = make(map[string]net.Conn)
	serverAddr := os.Args[2]
	fmt.Printf("serverAddr = %s \n", serverAddr)
	go func() {
		startServer(serverAddr)
	}()
	tailFile(logFileName)
}

func startServer(serverAddr string) {
	// TBD handle errors
	listen, err := net.Listen("tcp", serverAddr)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	// close listener
	defer listen.Close()
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		clientAddr := conn.RemoteAddr().String()
		fmt.Printf("clientAddr = %s \n", clientAddr)
		activeClients[clientAddr] = conn

		// send the client info on existing active connections
		sendActiveConnections(conn)
	}
}

func sendActiveConnections(conn net.Conn) {
	for _, info := range activeConnections {
		buffer, err := json.Marshal(info)
		if err != nil {
			fmt.Printf("err = %v \n", err)
			continue
		}
		_, err = conn.Write(buffer)
		if err != nil {
			fmt.Printf("err = %v \n", err)
			fmt.Printf("deleting: %s \n", info.LocalSourceIP)
			fmt.Printf("deleting connection for : %v \n", info.ConnectionToken)
			delete(activeConnections, info.ConnectionToken)
		}
	}
}

func tailFile(filepath string) {
	t, err := tail.TailFile(
		filepath,
		tail.Config{Follow: true, Location: &tail.SeekInfo{Offset: 0, Whence: 0}},
	)
	if err != nil {
		log.Fatalf("tail file err: %v", err)
	}

	// this loop runs forever, until interrupted, like tail -f
	for line := range t.Lines {
		s := line.Text
		if s != "" {
			substring := "[Start connection]"
			if strings.Contains(s, substring) {
				// parse the info and send to flp
				connInfo := parseLine(s)
				connInfo.ConnectionActive = true
				fmt.Printf("add connInfo = %v \n", connInfo)
				activeConnections[connInfo.ConnectionToken] = connInfo

				// send connection info to all registered clients
				sendConnInfo(connInfo)
			}
			substring = "[Close connection]"
			if strings.Contains(s, substring) {
				// parse the info and send to flp
				connInfo := parseLine(s)
				fmt.Printf("delete connInfo = %v \n", connInfo)
				delete(activeConnections, connInfo.ConnectionToken)
				connInfo.ConnectionActive = false
				sendConnInfo(connInfo)
			}
		}
	}
	fmt.Printf("no more lines \n")
}

func sendConnInfo(connInfo api.MbgConnectionInfo) {
	// convert connection info into JSON string
	buffer, err := json.Marshal(connInfo)
	if err != nil {
		log.Fatal(err)
		return
	}

	// send to each of the active clients
	for _, c := range activeClients {
		_, err := c.Write(buffer)
		if err != nil {
			fmt.Printf("err = %v \n", err)
			delete(activeClients, c.RemoteAddr().String())
			return
		}
	}
}

func parseLine(s string) api.MbgConnectionInfo {
	sArray := strings.SplitAfter(s, "wildcard:")
	sArray2 := strings.Split(sArray[1], " ")
	s1 := strings.Split(sArray2[0], "-")
	localSourceUrl := sArray2[2]
	s2 := strings.Split(localSourceUrl, ":")
	gatewayUrl := sArray2[8]
	gatewayUrl = strings.TrimRight(gatewayUrl, ")")
	gatewayUrl = strings.TrimLeft(gatewayUrl, "(")
	s3 := strings.Split(gatewayUrl, ":")
	c := api.MbgConnectionInfo{
		ServiceName:     s1[0],
		ConnectionToken: s1[1],
		LocalSourceIP:   s2[0],
		LocalSourcePort: s2[1],
		GatewayIP:       s3[0],
		GatewayPort:     s3[1],
		ExpireTime:      time.Now(),
	}
	fmt.Printf("c = %v \n", c)
	return c
}
