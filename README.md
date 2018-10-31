# chili-copy : fast and efficient remote file transfer
*Serves Your Files, Fresh and Hot !*

--------------------------------
## Overview
chili-copy is a parallel file transfer client-server software that efficiently transfers large files to remote machines over a network.
## Getting Started
### Building From Source
chili-copy is written in `Go`. To be able o build it on your machine. Please make sure `Go version 1.11` or higher is installed on your system and `$GOPATH` is appropriately is set.
#### Steps
Checkout the code in `$GOPATH/src`
```
# git clone https://github.com/Abhishekvrshny/chili-copy.git
```
You can also `go get` it
```
# go get -u github.com/Abhishekvrshny/chili-copy
```
Get into the root directory and run `make`
```
# cd chili-copy 
# make
```
This would generate two binaries in `./bin/`:
* `ccp_server`
* `cpp_client`

#### Download Binaries
The binaries can also be downloaded from https://github.com/Abhishekvrshny/chili-copy/releases and run directly. Please note that there are different binaries for `osx` and `linux` which are suffixed appropriately.
### Running the server
The `ccp_server -help`
```
# Usage of ./bin/ccp_server:
  -conn-size int
    	connection queue size (default 40)
  -port string
    	server port (default "5678")
  -worker-count int
    	count of worker threads (default 4)
```
***-conn-size*** : The queue size of the accepted connections. Default is number of CPUs x 10
***-port*** : The port on which to bind the server
***-worker-count*** : The number of worker threads that read from connection queue and process the requests. Default is number of CPUs on the system.
### Running the client
The `ccp_client -help`
```
Usage of ./bin/ccp_client:
  -chunk-size uint
    	multipart chunk size (bytes) (default 16777216)
  -destination-address string
    	destination server host and port (eg. localhost:5678)
  -local-file string
    	local file to copy
  -remote-file string
    	remote file at destination
  -worker-count int
    	count of worker threads (default 4)
```
***-chunk-size*** : This is used in 2 places. 1. To initiate multipart copy only if fileseize is greater than `chunk-size`. Also, in multipart copy, file is chunked sent to server in chunks of size `chunk-size`
***-destination-address*** : Server host and port where the copy is to be done.
***-local-file*** : Path of local file.
**-remote-file*** : Number of workers to send multipart chunks. default is number of CPUs on the system.
### Internals and Working of chili-copy
chili-copy is based on a custom-built binary protocol over TCP that is used to perform 2 types of transfer:
#### Single Copy Transfer
As part of single copy, following things happen:
1. Client identifies that the file size is less than chunk size and hence initiate single copy.
2. Client creates a TCP connection with the server.
3. Client sends a protocol header identifying single copy operation to the server
4. Server listens on the socket, accepts a connection and puts it on a connection queue.
5. A worker thread in server picks up the connection and reads initial 2 bytes from the protocol header to identify the type of operation.
6. Server identifies the type of operation as single copy and then reads the rest of the bytes from the protocol header to find the remote file path and content length.
7. Server adds the remote file path in a map, which would be used to prevent concurrent operations to same remote fie path on server.
8. Client sends the file over TCP socket to the servr
9. Server reads content-length number of bytes and writes them to the remote path specified.
10. Server sends the response back to the client with a success header and checksum of the file it recived.
11. Client reads initial 2 bytes of the response to identify the type of operation.
12. Client checks the checksum received and matches it with the local checksum and prints success else prints appropriate error. Error may also be recived from server. 

#### Multipart Copy Transfer
As part of multipart copy, client identifies that this is a multipart copy as file size is greater than chunk size and initiales following 3 types of operations in the same order
##### Initiate Multipart Copy
1. Cleint establishes a TCP connection and sends a header identifying init of a multipart copy.
2. Server generated and sends a unique copy-id as part of response header. It also adds the remote file path in a map, as described above.
3. Server also create and adds an entry into a map with copy-id as the key to identify forth coming operations, before sending the response to teh client.
3. Client creates meta info with fd, chunk size, offset etc and puts it in a job queue.
4. Client spawns multiple workers (equal to worker-count).
#### Multipart Copy Part
1. The workers on the client read the meta info and read the chunks from the fd specified by chunk size and offset. This happens in parallel by each worker independently.
2. Each worker now initiates a single copy of the part as described earlier.
3. Server identifies that it's a multipart copy part operation and creates a scratch directory where it keeps writing the chunks received by various workers. The format is `/tmp/<copy-id>/<part-num>`
4. Server keeps sending success for these parts received as described in single copy
5. The workers at client put the result in a result queue.
6. The main thread at client keeps reading the result queue until all the results are received.
#### Multipart Complete
1. After results for all the parts are received by the client, it initiates a multipart complete operation.
2. The server on recieving this operation, walks through the scratch directory and stitches all the parts and appends them together at the remote file at the server.
3. Server then sends a checksum of the resultant file as response to the client.
4. The client verifies the checksum and marks the copy as successful or failed.
