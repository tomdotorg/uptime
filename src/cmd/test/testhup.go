package main

import (
	"fmt"
	"os"
	"os/signal"
)

func main() {
	fmt.Println("making a channel")
	sigChannel := make(chan os.Signal, 1)
	fmt.Println("registering all signals")
	signal.Notify(sigChannel) // no second parameter means all signals
	fmt.Println("called notify. blocking for handler")
	for {
		s := <-sigChannel
		fmt.Println("***** Got signal: ", s)
	}
}
