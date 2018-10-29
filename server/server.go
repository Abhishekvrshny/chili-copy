package main

import (
	"fmt"
	"net"
	"os"

	"github.com/chili-copy/server/controller"
)

const (
	AcceptedConnQSize = 20
)
const (
	network = "tcp"
	address = ":5678"
)
func main() {
	fmt.Println("starting chili-copy server")
	cc := controller.NewChiliController()
	cc.MakeAcceptedConnQ(AcceptedConnQSize)
	cc.CreateAcceptedConnHandlers(AcceptedConnQSize)
	startChiliServer(cc, network, address)
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
