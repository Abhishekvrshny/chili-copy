MAKEFILE_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
SERVER_BINARY := $(MAKEFILE_DIR)/bin/ccp_server
CLIENT_BINARY := $(MAKEFILE_DIR)/bin/ccp_client

all:
	mkdir $(MAKEFILE_DIR)/bin/
	cd server && go build -o $(SERVER_BINARY) server.go
	cd client && go build -o $(CLIENT_BINARY) client.go

linux:
	cd server && GOARCH=amd64 GOOS=linux go build -o $(SERVER_BINARY)_linux_amd64 server.go
	cd client && GOARCH=amd64 GOOS=linux go build -o $(CLIENT_BINARY)_linux_amd64 client.go

clean:
	rm -rf $(MAKEFILE_DIR)/bin/*



