# chili-copy : fast and efficient remote file transfer
*Serves Your Files, Fresh and Hot !*

--------------------------------
## Overview
chili-copy is a parallel file transfer client-server software that efficiently transfers large files to remote machines over a network. It is implemented using a novel custom protocol, named as CCFTP, which is described in later sections. 
## Getting Started
### Building From Source
chili-copy is written in `Go`. To be able to build it on your machine, please make sure `Go version 1.11` or higher is installed on your system and `$GOPATH` is appropriately is set.
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
## Running the server
The `ccp_server -help`
```
# ./bin/ccp_server --help
Usage of ./bin/ccp_server:
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

## Running the client
The `ccp_client -help`
```
# ./bin/ccp_client --help
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

***-chunk-size*** : This is used in 2 places. First, to initiate multipart copy only if fileseize is greater than `chunk-size`. Also, in multipart copy, file is chunked and sent to server in chunks of size `chunk-size`. Default value is 16MB.

***-destination-address*** : Server host and port where the copy is to be done.

***-local-file*** : Path of local file.

***-remote-file*** : Number of workers to send multipart chunks. default is number of CPUs on the system.

## Internals and Working of chili-copy
chili-copy is based on a custom-built binary protocol over TCP that is used to perform 2 types of transfer:
### Single Copy Transfer
As part of single copy, following things happen:
1. Client identifies that the file size is less than chunk size and hence initiate single copy.
2. Client creates a TCP connection with the server.
3. Client sends a protocol header identifying single copy operation to the server
4. Server listens on the socket, accepts a connection and puts it on a connection queue.
5. A worker thread in server picks up the connection and reads initial 2 bytes from the protocol header to identify the type of operation.
6. Server identifies the type of operation as single copy and then reads the rest of the bytes from the protocol header to find the remote file path and content length.
7. Server adds the remote file path in a map, which would be used to prevent concurrent operations to same remote fie path on server.
8. Client sends the file over TCP socket to the server.
9. Server reads content-length number of bytes and writes them to the remote path specified.
10. Server sends the response back to the client with a success header and checksum of the file it received.
11. Client reads initial 2 bytes of the response to identify the type of operation.
12. Client checks the checksum received and matches it with the local checksum and prints success else prints appropriate error. 
13. Errors may also be received from server. 

### Multipart Copy Transfer
As part of multipart copy, client identifies that this is a multipart copy as file size is greater than chunk size and initiates following 3 types of operations in the same order.
#### Initiate Multipart Copy
1. Client establishes a TCP connection and sends a header identifying init of a multipart copy.
2. Server generates and sends a unique copy-id as part of response header.
3. Server also adds the remote file path in a map, as described above.
4. Server also create and adds an entry into a map with copy-id to identify forth coming operations, before sending the response to the client.
5. Client creates meta info with fd, chunk size, offset etc and puts it in a job queue.
6. Client spawns multiple workers (equal to worker-count).
### Multipart Copy Part
1. The workers on the client read the meta info and read the chunks from the fd specified by chunk size and offset. This happens in parallel by each worker independently.
2. Each worker now initiates a single copy of the part as described earlier.
3. Server identifies that it's a multipart copy part operation and creates a scratch directory where it keeps writing the chunks received by various workers. The format is `/tmp/<copy-id>/<part-num>`
4. Server keeps sending success for these parts received as described in single copy
5. The workers at client put the result in a result queue.
6. The main thread at client keeps reading the result queue until all the results are received.
### Multipart Complete
1. After results for all the parts are received by the client, it initiates a multipart complete operation.
2. The server on receiving this operation, walks through the scratch directory and stitches all the parts and appends them together at the remote file at the server.
3. Server then sends a checksum of the resultant file as response to the client.
4. The client verifies the checksum and marks the copy as successful or failed.

## Chili-Copy File Transfer Protocol (CCFTP)
chili-copy introduces a novel protocol to copy files in chunks, which is being named as CCFTP. CCFTP is a binary protocol that works over TCP. CCFTP works as follows:
1. The client establishes a connection with the server and sends CCFTP headers followed by data (file oe chunks of file)
2. The server reads the headers to identify the type of operation and performs appropriate actions.
3. The server sends back CCFTP header with results in the response

The CCFTP header is 512 bytes. Although, the actual bytes used by the protocol are always less than 300 which are used by various operations. The remaining bytes are padded with zeros before sending the headers across. CCFTP supports the following type of operations and headers.

### SingleCopyOpType

| | | | | |
|:-:|:-:|:-:|:-:|:-:|
| opcode<br>(2 bytes)   | filesize<br>(8 bytes)  | length of remote path string<br>(1 byte) | remote file path<br>(upto 255 bytes) | padding<br>(rest of 512 bytes) |

This is used by client to send a single copy request to the server, followed by the contents of the file.

### SingleCopySuccessResponseOpType

| | | |
|:-:|:-:|:-:|
| opcode<br>(2 bytes)   | file checksum<br>(16 bytes) | padding<br>(rest of 512 bytes) |

This is used by the server to send a successful single copy response to the client.

### MultiPartCopyInitOpType

| | | | |
|:-:|:-:|:-:|:-:|
| opcode<br>(2 bytes) | length of remote path string<br>(1 byte) | remote file path<br>(upto 255 bytes) | padding<br>(rest of 512 bytes) |

This is sent by client to initiate a multipart copy.

### MultiPartCopyInitSuccessResponseOpType

| | | |
|:-:|:-:|:-:|
| opcode<br>(2 bytes) | copy id<br>(16 bytes) | padding<br>(rest of 512 bytes) |

This is sent by the server in response to a successful multipart init request.

### MultiPartCopyPartRequestOpType

| | | | | |
|:-:|:-:|:-:|:-:|:-:|
| opcode<br>(2 bytes) | copy id<br>(16 bytes) | part number<br>(8 bytes) | part size<br>(8 bytes) |padding<br>(rest of 512 bytes) |

This is sent by the client to send a file chunk to the server.

### MultiPartCopyCompleteOpType

| | | | |
|:-:|:-:|:-:|:-:|
| opcode<br>(2 bytes) | copy id<br>(16 bytes) | file size<br>(8 bytes) |padding<br>(rest of 512 bytes) |

This is sent by the client to complete a multipart copy operation after all chunks are sent by it.

### MultiPartCopySuccessResponseOpType

| | | |
|:-:|:-:|:-:|
| opcode<br>(2 bytes)   | file checksum<br>(16 bytes) | padding<br>(rest of 512 bytes) |

This is used by the server to send a successful multipart copy response to the client. The structure is similar to that of SingleCopySuccessResponseOpType, with just opcode being different.

### ErrorResponseOpType

| | | |
|:-:|:-:|:-:|
| opcode<br>(2 bytes)   | error type<br>(1 byte) | padding<br>(rest of 512 bytes) |  padding<br>(rest of 512 bytes) |

This is used by server to send various errors to the client.

## TODOs

* Use `sendfile()` to directly send file to the socket without reading in userspace, to enhance performance.
* The `protocol` package can be refactored to make it more intuitive.
* Handshake can be introduced between server and client to determine right amount of parallelism.
* Stitching logic can be optimised at server. right now 2x space is needed in this process. It could be done with a constant small buffer size.
* Unit tests are completely missing as of now.
* Perform thorough benchmarks

## Chili-Copy in Action

[![chili-copy](http://img.youtube.com/vi/Nzc3WpUjiOE/0.jpg)](https://youtu.be/Nzc3WpUjiOE "chili-copy")

## Quick Benchmark with scp

A quick benchmark with scp was done. For a ~85MB file, chili-copy was ~65% faster than scp, with a chunk-size of 4MB. 

```
⋊> ~/g/s/g/chili-copy on master ⨯ time ./bin/ccp_client -destination-address=10.33.121.238:5678 -remote-file=/tmp/abc -local-file=/Users/abhishek.varshney/Downloads/go1.10.4.darwin-amd64.tar.gz  -worker-count=6 -chunk-size=4194304
chili-copy client
Request : multipart copy : /Users/abhishek.varshney/Downloads/go1.10.4.darwin-amd64.tar.gz to 10.33.121.238:5678:/tmp/abc : size=90700370, csum@client=4497a2c528d8fcd9e1bad7c77fbc01df
CopyId received from server : 31c031d1-dd44-11e8-baf3-02010a2179ee
Total fileSize :  90700370
Total # of parts :  22
Response : successfully uploaded chunk # 2
Response : successfully uploaded chunk # 4
Response : successfully uploaded chunk # 1
Response : successfully uploaded chunk # 8
Response : successfully uploaded chunk # 7
Response : successfully uploaded chunk # 11
Response : successfully uploaded chunk # 3
Response : successfully uploaded chunk # 10
Response : successfully uploaded chunk # 13
Response : successfully uploaded chunk # 12
Response : successfully uploaded chunk # 9
Response : successfully uploaded chunk # 6
Response : successfully uploaded chunk # 5
Response : successfully uploaded chunk # 14
Response : successfully uploaded chunk # 16
Response : successfully uploaded chunk # 15
Response : successfully uploaded chunk # 20
Response : successfully uploaded chunk # 22
Response : successfully uploaded chunk # 19
Response : successfully uploaded chunk # 21
Response : successfully uploaded chunk # 18
Response : successfully uploaded chunk # 17
Successfully copied 22 chunks out of 22
Response : successfully copied : /Users/abhishek.varshney/Downloads/go1.10.4.darwin-amd64.tar.gz to 10.33.121.238:5678:/tmp/abc : size=90700370, csum@server=4497a2c528d8fcd9e1bad7c77fbc01df
       29.39 real         0.67 user         0.91 sys
⋊> ~/g/s/g/chili-copy on master ⨯ time scp /Users/abhishek.varshney/Downloads/go1.10.4.darwin-amd64.tar.gz 10.33.121.238:                                                                                                             01:06:54
go1.10.4.darwin-amd64.tar.gz                                                                                                                                                                                100%   86MB   1.7MB/s   00:51
       52.64 real         1.62 user         0.90 sys
⋊> ~/g/s/g/chili-copy on master ⨯                                                                                                                                                                                                     01:08:05```

