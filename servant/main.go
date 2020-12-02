package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const BUFFERSIZE = 1024
const HEADERSIZE = 82 //size of header for our particular p2p server

type RegistryStatus struct {
	status bool
	mux    sync.Mutex
}

func main() {
	args := os.Args[1:]
	header := make([]byte, HEADERSIZE)
	fileDirectory, _ := os.Open("tmp")
	listOfFiles, _ := fileDirectory.Readdirnames(0)
	fileDirectory.Close()
	connection, err := net.Dial("tcp", args[0])
	raddr := connection.RemoteAddr().String()
	laddr := connection.LocalAddr().String()
	if err != nil {
		panic(err)
	}
	defer connection.Close()
	fmt.Println("Connected to registry")

	registryStatus := RegistryStatus{
		status: false,
		mux:    sync.Mutex{},
	}
	/*Here is where we process arguments and initialize the header which
	contains info of what the servant wants to say to the registry*/
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-rf": //-rf means request file and is followed by the name of the file
			header[0] = 0x01 //Set rf flag in header to 1, which means servant wants a file
		case "-sf": //-sf means share files, and is used when servant wants to add their own files to the registry
			header[65] = 0x01                                                 //Set sf flag to 1 if we want to share files
			binary.BigEndian.PutUint32(header[66:], uint32(len(listOfFiles))) //include number of files that servants owns
			registryStatus.mux.Lock()
			registryStatus.status = true
			registryStatus.mux.Unlock()
		default:
			//this is were we process the file name arg that we are requesting from the registry
			requestedFileAsString := []byte(fillString(args[i], 64))
			for i, v := range requestedFileAsString {
				header[i+1] = v
			}
		}
	}
	go maintainRegistryStatus(raddr, laddr, &registryStatus)
	sendInfoToRegistry(connection, header)
	fucker := false
	for fucker == false {
		if fucker == true {
			fucker = true
		}
	}
}

/* Sends info header to the registry. The info header contains the
file request(if any) of the servant as well as the number of files
the servant is providing to the registry*/
func sendInfoToRegistry(conn net.Conn, info []byte) {
	fmt.Printf("Sending info to Registry as address %s\n", conn.RemoteAddr().String())
	conn.Write(info)
	registryApproval := make([]byte, 1)
	conn.Read(registryApproval)
	switch info[65] {
	case 0x01:
		fileDirectory, _ := os.Open("tmp")
		listOfFiles, _ := fileDirectory.Readdirnames(0)
		for _, name := range listOfFiles {
			name = fillString(name, 64)
			conn.Write([]byte(name))
		}
	}
}

func maintainRegistryStatus(raddr string, laddr string, s *RegistryStatus) {
	udpladdr, err := net.ResolveUDPAddr("udp", laddr)
	if err != nil {
		panic(err)
	}
	udpraddr, err := net.ResolveUDPAddr("udp", raddr)
	if err != nil {
		panic(err)
	}
	conn, err := net.DialUDP("udp", udpladdr, udpraddr)
	if err != nil {
		panic(err)
	}

	s.mux.Lock()
	status := s.status
	s.mux.Unlock()

	for status == true {
		conn.Write([]byte("hello"))
		time.Sleep(2 * time.Second)
		s.mux.Lock()
		status = s.status
		s.mux.Unlock()
	}
}

func sendFileToClient(connection net.Conn, fileToSendName string) {
	fmt.Println("A client has connected!")
	defer connection.Close()
	file, err := os.Open(fileToSendName)
	if err != nil {
		fmt.Println(err)
		return
	}
	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Println(err)
		return
	}
	fileSize := fillString(strconv.FormatInt(fileInfo.Size(), 10), 10)
	fileName := fillString(fileInfo.Name(), 64)
	fmt.Println("Sending filename and filesize!")
	connection.Write([]byte(fileSize))
	connection.Write([]byte(fileName))
	sendBuffer := make([]byte, BUFFERSIZE)
	fmt.Println("Start sending file!")
	for {
		_, err = file.Read(sendBuffer)
		if err == io.EOF {
			break
		}
		connection.Write(sendBuffer)
	}
	fmt.Println("File has been sent, closing connection!")
	return
}

func fillString(retunString string, toLength int) string {
	for {
		lengtString := len(retunString)
		if lengtString < toLength {
			retunString = retunString + ":"
			continue
		}
		break
	}
	return retunString
}

func receiveFileFromServant(connection net.Conn) {
	bufferFileName := make([]byte, 64)
	bufferFileSize := make([]byte, 10)

	connection.Read(bufferFileSize)
	fileSize, _ := strconv.ParseInt(strings.Trim(string(bufferFileSize), ":"), 10, 64)

	connection.Read(bufferFileName)
	fileName := strings.Trim(string(bufferFileName), ":")

	newFile, err := os.Create(fileName)

	if err != nil {
		panic(err)
	}
	defer newFile.Close()
	var receivedBytes int64

	for {
		if (fileSize - receivedBytes) < BUFFERSIZE {
			io.CopyN(newFile, connection, (fileSize - receivedBytes))
			connection.Read(make([]byte, (receivedBytes+BUFFERSIZE)-fileSize))
			break
		}
		io.CopyN(newFile, connection, BUFFERSIZE)
		receivedBytes += BUFFERSIZE
	}
	fmt.Println("Received file completely!")
}
