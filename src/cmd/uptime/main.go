package main

import (
	"fmt"
	"os"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"upcheck"
)

const CONFIGFILE = "hosts.txt"

func main() {
	initLogs()
	netInfo, err := upcheck.GetNetworkInfo()

	if err != nil {
		log.Fatal().Err(err).Msg("Error getting network info")
	} else {
		fmt.Printf("Local IP: %s\n", netInfo.Localnet)
		fmt.Printf("Netmask: %s\n", upcheck.IPMaskToString(netInfo.Mask))
		fmt.Printf("Default Gateway: %s\n", netInfo.GW)
	}

	checkTargets := upcheck.LoadTargets(CONFIGFILE)
	// go showStatuses(checkTargets)

	// Initialize keyboard listener
	if err := keyboard.Open(); err != nil {
		log.Fatal().Err(err).Msg("Failed to open keyboard")
	}

	defer func() {
		if err := keyboard.Close(); err != nil {
			log.Fatal().Err(err).Msg("Failed to close keyboard")
		}
	}()
	stopChan := make(chan struct{})
	go func(stopChan chan struct{}) {
		for {
			select {
			case <-stopChan:
				log.Info().Msg("Stopping...")
				return
			default:
				upcheck.CheckAllTargets(checkTargets)
				time.Sleep(1 * time.Second)
			}

		}
	}(stopChan)
	for {
		handleKeys(checkTargets, stopChan)
	}
}

func handleKeys(checkTargets []*upcheck.Target, stopChan chan struct{}) []*upcheck.Target {
	// Check for key presses
	if char, key, err := keyboard.GetKey(); err == nil {
		if key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			fmt.Println("Exiting...")
			os.Exit(0)
		}
		switch char {
		case 'q':
			fmt.Println("Exiting...")
			os.Exit(0)
		case 's':
			upcheck.ShowStatuses(checkTargets)
			break
		case 'r':
			fmt.Println("Resetting all stats...")
			upcheck.ResetAllStats(checkTargets)
		default:
			fmt.Printf("You pressed: %q\n", char)
		}
	}
	return checkTargets
}

func initLogs() {
	// initialize the logger
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// if os.Getenv("CONSOLE") != "" || 1 == 1 {
	//	log.Info().Msg("logging to console")
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// } else {
	//	log.Output(os.Stdout)
	// }

	if os.Getenv("DEBUG") != "" {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
		log.Info().Msg("enabling Trace level logging")
	} else {
		log.Info().Msg("enabling Info level logging")
	}
}
