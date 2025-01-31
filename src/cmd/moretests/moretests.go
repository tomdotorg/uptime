package main

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"os/signal"
	"syscall"
)

func multiSignalHandler(signal os.Signal) {

	switch signal {
	case syscall.SIGHUP:
		fmt.Println("Signal:", signal.String())
		//		os.Exit(0)
	case syscall.SIGKILL:
		fmt.Println("Signal:", signal.String())
		os.Exit(0)
	case syscall.SIGINT:
		fmt.Println("Signal:", signal.String())
		os.Exit(0)
	case syscall.SIGTERM:
		fmt.Println("Signal:", signal.String())
		os.Exit(0)
	case syscall.SIGQUIT:
		fmt.Println("Signal:", signal.String())
		os.Exit(0)
	default:
		fmt.Println("Unhandled/unknown signal: ", signal.String())
	}
}

func main() {
	sigchnl := make(chan os.Signal, 1)
	signal.Notify(sigchnl) //we can add more sycalls.SIGQUIT etc.
	exitchnl := make(chan int)

	go func() {
		for {
			log.Info().Msg("looping.. blocking on sigchnl")
			s := <-sigchnl
			log.Info().Msg("got a signal. sending it to the handler")
			multiSignalHandler(s)
		}
	}()

	log.Info().Msg("done looping.. blocking on exitchnl")
	exitcode := <-exitchnl
	os.Exit(exitcode)
}
