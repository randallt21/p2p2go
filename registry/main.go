package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

//BUFFERSIZE is length for sending packets
const BUFFERSIZE = 1024

//HEADERSIZE is size of info header for our particular p2p server
const HEADERSIZE = 82

/* HEADER FORMAT:
82 BYTE TOTAL
Byte 0 -> rf flag(0 for no rf, 1 for rf)
Byte 1-64 -> name of file being requested as byte array, all zero if no file requested
Byte 65 -> sf flag(0 for not sharing files, 1 for sharing)
Byte 66-81 -> contains number of files that servant has to share, 0 is servant not sharing
*/
type servantFile struct {
	name  string
	owner string
}
type listOfFiles struct {
	list []servantFile
	mux  sync.Mutex
}
type listOfServants struct {
	list []string
	mux  sync.Mutex
}

func main() {

	//list of servants who have made their files available to the registry
	listOfServants := listOfServants{
		list: make([]string, 0),
		mux:  sync.Mutex{},
	}
	listOfFiles := listOfFiles{
		list: make([]servantFile, 0),
		mux:  sync.Mutex{},
	}
	//Address of this p2p registry
	registryAddress := net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8080,
		Zone: "",
	}

	//Start listening for incoming servants
	server, err := net.ListenTCP("tcp", &registryAddress)
	if err != nil {
		fmt.Println("Error listening: ", err)
		os.Exit(1)
	}
	defer server.Close()

	fmt.Print("Registry open, waiting for servants to appear...\n\n")

	for {
		connection, err := server.AcceptTCP()     // Servant found!
		raddr := connection.RemoteAddr().String() // Get address of remote servant
		laddr := connection.LocalAddr().String()

		if err != nil {
			fmt.Println("Error: ", err)
			os.Exit(1)
		}

		fmt.Printf("Servant at address %s has appeared\n", raddr)

		go handleServant(connection, &listOfServants, &listOfFiles)  //Thread for handling servant
		go trackServant(raddr, laddr, &listOfServants, &listOfFiles) // Thread for updating availability of servant files
	}
}

// This function is for checking if a servant is active and responsive to the registry.
// If a servant fails to send a udp hello message to the server every 200 seconds than the
// servant is dropped from the list of available servants.
func trackServant(raddr string, laddr string, ls *listOfServants, lf *listOfFiles) {
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
	if err != nil {
		panic(err)
	}
	conn.SetReadDeadline(time.Now().Add(time.Second * 20))
	response := make([]byte, 5)
	for {
		_, err := conn.Read(response)
		if err != nil {
			ls.mux.Lock()
			for j := 0; j < len(ls.list); j++ {
				if ls.list[j] == raddr && j != 0 {
					ls.list = append(ls.list[0:j], ls.list[j+1:]...)
					fmt.Printf("%s has been removed from list of servants\n", raddr)
				} else if ls.list[j] == raddr && j == 0 {
					ls.list = ls.list[1:]
					fmt.Printf("%s has been removed from list of servants\n", raddr)
				}
			}
			ls.mux.Unlock()
			lf.mux.Lock()
			for i := 0; i < len(lf.list); i++ {
				if lf.list[i].owner == raddr {
					lf.list = append(lf.list[:i], lf.list[i+1:]...)
					i--
				}
			}
			lf.mux.Unlock()
			break
		}
	}
}

// This function is where we will put code that process a particular servants request, such as
// requesting some file that may exist in the registry list.
func handleServant(connection net.Conn, ls *listOfServants, lf *listOfFiles) {
	servantInfo := make([]byte, HEADERSIZE)
	bufferFileName := make([]byte, 64)
	connection.Read(servantInfo)
	connection.Write([]byte{1})
	switch servantInfo[65] {
	case 0x01:
		ls.mux.Lock()
		ls.list = append(ls.list, connection.RemoteAddr().String())
		ls.mux.Unlock()
		numberOfFiles := int(binary.BigEndian.Uint32(servantInfo[66:]))
		for i := 0; i < numberOfFiles; i++ {
			connection.Read(bufferFileName)
			availableFile := servantFile{
				name:  strings.Trim(string(bufferFileName), ":"),
				owner: connection.RemoteAddr().String(),
			}
			lf.mux.Lock()
			lf.list = append(lf.list, availableFile)
			lf.mux.Unlock()
		}
	}
}
