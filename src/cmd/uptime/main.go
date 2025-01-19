package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rivo/tview"

	"github.com/eiannone/keyboard"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"upcheck"
)

type RunInfo struct {
	programStartedTime *time.Time
	networkInfo        *upcheck.NetworkInfo
	configTime         *time.Time
	checkTargets       []*upcheck.Target
	configFilename     *string
	interval           *int
}

var app *tview.Application

const CONFIGFILE = "hosts.txt"

func main() {
	runInfo := RunInfo{
		programStartedTime: func() *time.Time { t := time.Now(); return &t }(),
	}
	// app = tview.NewApplication()
	// textView := tview.NewTextView().
	// 	SetText("Hello, world!").
	// 	SetTextAlign(tview.AlignCenter).
	// 	SetDynamicColors(true)

	// if err := app.SetRoot(textView, true).Run(); err != nil {
	// 	panic(err)
	// }
	// Define command line flags
	runInfo.configFilename = flag.String("f", CONFIGFILE, "Filename containing the targets")
	runInfo.interval = flag.Int("i", 2, "Number of seconds between target checks")

	// Parse the command line flags
	flag.Parse()

	initLogs()

	netInfo, err := upcheck.GetNetworkInfo()

	if err != nil {
		log.Fatal().Err(err).Msg("Error getting network info")
	} else {
		runInfo.networkInfo = &netInfo
		fmt.Printf("Local IP: %s\n", netInfo.Localnet)
		fmt.Printf("Netmask: %s\n", upcheck.IPMaskToString(netInfo.Mask))
		fmt.Printf("Default Gateway: %s\n", netInfo.GW)
		fmt.Println()
	}

	runInfo.checkTargets = upcheck.LoadTargets(*runInfo.configFilename)
	if upcheck.FindDefaultGateway(runInfo.checkTargets, netInfo) == nil {
		log.Info().Msgf("Default gateway %s not in targets adding it", netInfo.GW)
		runInfo.checkTargets = upcheck.AddDefaultGatewayTarget(runInfo.checkTargets, netInfo)
	}

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
	go loopCheckAllTargets(runInfo.checkTargets, runInfo.interval)(cmdChan)
	for {
		handleKeys(runInfo.checkTargets, cmdChan, runInfo)
	}
}

func loopCheckAllTargets(checkTargets []*upcheck.Target, interval *int) func(cmdChan chan string) {
	return func(cmdChan chan string) {
		ticker := time.NewTicker(time.Duration(*interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case cmd := <-cmdChan:
				if cmd == "stop" {
					log.Debug().Msg("Stopping...")
					ticker.Stop()
					return
				}
			case <-ticker.C:
				log.Debug().Msg("Checking all targets")
				upcheck.CheckAllTargets(checkTargets)
			}
		}
	}
}

func handleKeys(checkTargets []*upcheck.Target, cmdChan chan string, runInfo RunInfo) []*upcheck.Target {
	// Check for key presses
	if char, key, err := keyboard.GetKey(); err == nil {
		if key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			fmt.Println("Exiting...")
			os.Exit(0)
		}
		switch char {
		case 'q':
			fmt.Println("Exiting...")
			cmdChan <- "stop"
			os.Exit(0)
		case 's':
			upcheck.ShowStatuses(checkTargets)
			break
		case 'u':
			fmt.Println("Resuming...")
			go loopCheckAllTargets(checkTargets, runInfo.interval)(cmdChan)
		case 'p':
			fmt.Println("Pausing...")
			cmdChan <- "stop"
		case 'r':
			fmt.Println("Resetting all stats...")
			upcheck.ResetAllStats(checkTargets)
		case 'x':
			fmt.Println("Stopping...")
			cmdChan <- "stop"
			fmt.Println("Stopped")
			fmt.Println("Resetting all stats...")
			upcheck.ResetAllStats(checkTargets)
			fmt.Println("Reset all stats...")
			upcheck.ShowStatuses(checkTargets)
			fmt.Println("Starting...")
			go loopCheckAllTargets(checkTargets, runInfo.interval)(cmdChan)
			fmt.Println("Started")
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
