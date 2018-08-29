package astutil

import (
	"net"
	"fmt"
)

func doRead() {
	conn, err := net.Dial("tcp", "golang.org:80")
	if err != nil {
		// handle error
	}
	n, _ := conn.Read([]byte{})
	fmt.Println(n)
	conn, err = net.Dial("udp4", "golang.org:80")
	if err != nil {
		// handle error
	}
	n, _ = conn.Read([]byte{})
}
