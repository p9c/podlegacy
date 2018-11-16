package main

import (
	"log"
	"net"
	"os"
	"time"

	"github.com/lightninglabs/gozmq"
)

func main() {
	c, err := gozmq.Subscribe(os.Args[1], []string{""}, time.Second)
	if err != nil {
		log.Fatalf("gozmq.Subscribe error: %v", err)
	}

	for {
		msg, err := c.Receive()
		if err != nil {
			switch e := err.(type) {
			case net.Error:
				if e.Timeout() {
					continue
				}
				log.Fatalf("Receive error: %v", err)
			default:
				log.Fatalf("Receive error: %v", err)
			}
		}
		log.Printf("Received message: %+v", msg)
	}
}
