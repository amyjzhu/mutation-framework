package main

import (
	"./compositions"
	"log"
	"time"
)

func main() {
	nc, err := compositions.InitializeCapture("./net_config.json", 30 * time.Second)
	if err != nil {
		log.Fatal(err)
	}
	nc.StartCapture()
	time.Sleep(60 * time.Second)
	nc.StopCapture()
	log.Println(nc.Stats)
}
