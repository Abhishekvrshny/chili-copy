package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/chili-copy/server/controller"
	"runtime"
)

const (
	network = "tcp"
)

func main() {
	port, ConnQSize, workerThreads := getCmdArgs()
	cc := controller.NewChiliController()
	cc.MakeAcceptedConnQ(*ConnQSize)
	cc.CreateAcceptedConnHandlers(*workerThreads)
	fmt.Printf("starting chili-copy server on port %s\n", port)
	startChiliServer(cc, network, port)
}

func getCmdArgs() (string, *int, *int) {
	var port string
	flag.StringVar(&port, "port", "5678", "server port")
	ConnQSize := flag.Int("conn-size", runtime.NumCPU()*10, "connection queue size")
	workerThreads := flag.Int("worker-count", runtime.NumCPU(), "count of worker threads")

	flag.Parse()
	port = fmt.Sprintf(":%s", port)

	return port, ConnQSize, workerThreads
}

func startChiliServer(cc *controller.ChiliController, network string, port string) {
	ln, err := net.Listen(network, port)
	if err != nil {
		fmt.Printf("Unable to start server on port %s. Failed with error : %s\n", port, err.Error())
		os.Exit(1)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("Unable to accept connection. Failed with error : %s\n", err.Error())
			os.Exit(2)
		}
		cc.AddConnToQ(conn)
	}
}
