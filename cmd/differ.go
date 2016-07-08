package main

import (
	"log"
	"time"

	"github.com/sh3rp/differ"
)

func main() {
	executor := differ.NewExecutor()
	executor.StartPoller()
	id := executor.Track("192.168.1.202", "cat test.txt", 30)
	for {
		time.Sleep(1 * time.Second)
		for i, v := range executor.History(id) {
			log.Printf("History %d (%x): %v\n", i, string(v.Hash), string(v.Result))
		}
	}
}
