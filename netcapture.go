package main

import (
	"./compositions"
	"fmt"
	"log"
	"time"
)

func main() {
	nr, err := compositions.InitializeNodeRoles("./node_roles.json")
	if err != nil {
		log.Fatal(err)
	}
	if nr != nil {
		fmt.Println(nr)
	}
	nc, err := compositions.InitializeCapture("./net_config.json", 30 * time.Second)
	if err != nil {
		log.Fatal(err)
	}
	nc.StartCapture()
	time.Sleep(60 * time.Second)
	nc.StopCapture()
	log.Println(nc.Stats)
}
