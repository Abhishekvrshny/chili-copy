package main

import (
	"fmt"
	"net"
	"os"

	"github.com/chili-copy/server/controller"
)

//var acceptedConns chan net.Conn

func main() {
	fmt.Println("starting chili-copy server")
	cc := controller.NewChiliController()
	cc.MakeAcceptedConnQ(20)
	cc.CreateAcceptedConnHandlers(20)
	startChiliServer(cc, "tcp", ":5678")
}

func startChiliServer(cc *controller.ChiliController, network string, port string) {
	ln, err := net.Listen(network, port)
	if err != nil {
		os.Exit(1)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		cc.AddConnToQ(conn)
	}
}
