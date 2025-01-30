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
	paused             *bool
	app                *tview.Application
}

const CONFIGFILE = "hosts.txt"

func main() {
	runInfo := RunInfo{
		programStartedTime: func() *time.Time { t := time.Now(); return &t }(),
		app:                tview.NewApplication(),
		configFilename:     flag.String("f", CONFIGFILE, "Filename containing the targets"),
		interval:           flag.Int("i", 2, "Number of seconds between target checks"),
		paused:             new(bool),
	}
	*runInfo.paused = false
	// Parse the command line flags
	flag.Parse()

	// textView := tview.NewTextView().
	// 	SetText("Hello, world!").
	// 	SetTextAlign(tview.AlignCenter).
	// 	SetDynamicColors(true)

	// if err := app.SetRoot(textView, true).Run(); err != nil {
	// 	panic(err)
	// }
	// Define command line flags

	initLogs()

	netInfo, err := upcheck.GetNetworkInfo()

	if err != nil {
		log.Fatal().Err(err).Msg("Error getting network info")
	} else {
		runInfo.networkInfo = &netInfo
		fmt.Printf("Local IP: %s\n", netInfo.Address)
		fmt.Printf("Netmask: %s\n", upcheck.IPMaskToString(netInfo.Mask))
		fmt.Printf("Default Gateway: %s\n", netInfo.GW)
		fmt.Println()
	}

	runInfo.checkTargets = upcheck.LoadTargets(*runInfo.configFilename)
	showTargets(runInfo)

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
	go loopCheckAllTargets(&runInfo, cmdChan)
	for {
		handleKeys(&runInfo, cmdChan)
	}
}

func showTargets(runInfo RunInfo) {
	fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
	if runInfo.networkInfo == nil {
		fmt.Println("No network info available")
	} else {
		fmt.Printf("\nNetwork Config:\n%v\n\n", *runInfo.networkInfo)
	}
	subnetTargets, gatewayTargets, externalTargets, ok := upcheck.ClassifyTargets(runInfo.checkTargets, runInfo.networkInfo)
	if ok {
		upcheck.ShowStatuses("Subnet Targets", subnetTargets)
		upcheck.ShowStatuses("Gateway Targets", gatewayTargets)
		upcheck.ShowStatuses("External Targets", externalTargets)
	} else {
		upcheck.ShowStatuses("All Targets", runInfo.checkTargets)
	}
}

func loopCheckAllTargets(runInfo *RunInfo, cmdChan chan string) {
	ticker := time.NewTicker(time.Duration(*runInfo.interval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case cmd := <-cmdChan:
			switch cmd {
			case "stop":
				log.Debug().Msg("Stopping...")
				ticker.Stop()
				return
			}
		case <-ticker.C:
			// check for a network at all
			if !upcheck.HasNetworkConnection() {
				if runInfo.networkInfo != nil {
					log.Warn().Msg("No network connection detected - skipping checks")
					upcheck.MarkAllTargetsOffline(runInfo.checkTargets)
				}
				runInfo.networkInfo = nil
				continue
			} else {
				if runInfo.networkInfo == nil {
					runInfo.networkInfo = &upcheck.NetworkInfo{}
				}
			}
			// check for a network change
			newNetInfo, err := upcheck.GetNetworkInfo()
			if err != nil || runInfo.networkInfo == nil {
				if runInfo.networkInfo != nil {
					runInfo.networkInfo = nil
					log.Error().Err(err).Msg("Error getting network info after change - we must be offline")
				}
			} else if !runInfo.networkInfo.Equals(newNetInfo) {
				log.Warn().Msg("Network change detected")
				runInfo.networkInfo = &newNetInfo
				fmt.Printf("Network info: %v\n", newNetInfo)
				log.Info().Msg("Ensuring default gateway is in targets")
				if upcheck.FindDefaultGateway(runInfo.checkTargets, runInfo.networkInfo) == nil {
					runInfo.checkTargets = upcheck.AddDefaultGatewayTarget(runInfo.checkTargets, runInfo.networkInfo)
				}
				log.Debug().Msg("Checking all targets")
				upcheck.CheckAllTargets(runInfo.checkTargets)
				showTargets(*runInfo)
			}
			log.Debug().Msg("Checking all targets")
			upcheck.CheckAllTargets(runInfo.checkTargets)
		}
	}
}

func handleKeys(runInfo *RunInfo, cmdChan chan string) []*upcheck.Target {
	// Check for key presses
	if char, key, err := keyboard.GetKey(); err == nil {
		if key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			fmt.Println("Exiting...")
			cmdChan <- "stop"
			os.Exit(0)
		}
		switch char {
		case 'q', 'x':
			fmt.Println("Exiting...")
			os.Exit(0)
		case 's':
			showTargets(*runInfo)
			break
		case 'p':
			if *runInfo.paused {
				fmt.Println("Resuming...")
				*runInfo.paused = false
				go loopCheckAllTargets(runInfo, cmdChan)
			} else {
				fmt.Println("Pausing...")
				cmdChan <- "stop"
				*runInfo.paused = true
			}
		case 'r':
			fmt.Println("Resetting all stats...")
			upcheck.ResetAllStats(runInfo.checkTargets)
		case '?':
			showHelp()
		default:
			fmt.Printf("You pressed: %q\n", char)
		}
	}
	return runInfo.checkTargets
}

func showHelp() {
	fmt.Println("upcheck - a simple network uptime checker")
	fmt.Println("flags:" +
		"\n  -f <filename> : Filename containing the targets" +
		"\n  -i <interval> : Number of seconds between target checks")
	fmt.Println("Commands:")
	fmt.Println("  q or x: quit")
	fmt.Println("  s: show targets")
	fmt.Println("  p: pause/resume")
	fmt.Println("  r: reset all stats")
	fmt.Println("   input file format (one per line): 8.8.4.4:53")
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
