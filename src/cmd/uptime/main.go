package main

import (
	"flag"
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
	// Define command line flags
	filename := flag.String("f", CONFIGFILE, "Filename containing the targets")
	interval := flag.Int("i", 2, "Number of seconds between target checks")

	// Parse the command line flags
	flag.Parse()

	initLogs()

	netInfo, err := upcheck.GetNetworkInfo()

	if err != nil {
		log.Fatal().Err(err).Msg("Error getting network info")
	} else {
		fmt.Printf("Local IP: %s\n", netInfo.Localnet)
		fmt.Printf("Netmask: %s\n", upcheck.IPMaskToString(netInfo.Mask))
		fmt.Printf("Default Gateway: %s\n", netInfo.GW)
		fmt.Println()
	}

	checkTargets := upcheck.LoadTargets(*filename)
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
	cmdChan := make(chan string)
	go func(cmdChan chan string) {
		for {
			select {
			case cmd := <-cmdChan:
				switch cmd {
				case "stop":
					log.Info().Msg("Stopping...")
					return
				case "reset":
					log.Info().Msg("Resetting all stats...")
					upcheck.ResetAllStats(checkTargets)
				default:
					log.Info().Msg("Invalid command: " + cmd)
				}
			default:
				upcheck.CheckAllTargets(checkTargets)
				time.Sleep(time.Duration(*interval) * time.Second)
			}

		}
	}(cmdChan)
	for {
		handleKeys(checkTargets, cmdChan)
	}
}

func handleKeys(checkTargets []*upcheck.Target, cmdChan chan string) []*upcheck.Target {
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
