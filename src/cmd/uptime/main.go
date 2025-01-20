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
	subnetTargets, gatewayTargets, externalTargets := classifyTargets(runInfo.checkTargets, runInfo.networkInfo)
	fmt.Println("Subnet Targets:")
	upcheck.ShowStatuses(subnetTargets)
	fmt.Println("Gateway Targets:")
	upcheck.ShowStatuses(gatewayTargets)
	fmt.Println("External Targets:")
	upcheck.ShowStatuses(externalTargets)

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
			// check for a network change
			newNetInfo, err := upcheck.GetNetworkInfo()
			if err != nil {
				log.Error().Err(err).Msg("Error getting network info after change")
			} else if !runInfo.networkInfo.Equals(newNetInfo) {
				log.Warn().Msg("Network change detected")
				runInfo.networkInfo = &newNetInfo
				fmt.Printf("New Local IP: %s\n", newNetInfo.Address)
				fmt.Printf("New Netmask: %s\n", upcheck.IPMaskToString(newNetInfo.Mask))
				fmt.Printf("New Default Gateway: %s\n", newNetInfo.GW)
				log.Info().Msg("Reloading targets")
				runInfo.checkTargets = upcheck.LoadTargets(*runInfo.configFilename)
				subnetTargets, gatewayTargets, externalTargets := classifyTargets(runInfo.checkTargets, runInfo.networkInfo)
				fmt.Println("Subnet Targets:")
				upcheck.ShowStatuses(subnetTargets)
				fmt.Println("Gateway Targets:")
				upcheck.ShowStatuses(gatewayTargets)
				fmt.Println("External Targets:")
				upcheck.ShowStatuses(externalTargets)
			}
			log.Debug().Msg("Checking all targets")
			upcheck.CheckAllTargets(runInfo.checkTargets)
		}
	}
}

// classify the targets as on this subnet, gateway, or external to this subnet
func classifyTargets(targets []*upcheck.Target, netInfo *upcheck.NetworkInfo) (subnetTargets, gatewayTargets, externalTargets []*upcheck.Target) {
	for _, target := range targets {
		if target.IP.Equal(netInfo.GW) {
			gatewayTargets = append(gatewayTargets, target)
		} else if upcheck.IsInSameSubnet(netInfo.Address, netInfo.Mask, target.IP) {
			subnetTargets = append(subnetTargets, target)
		} else {
			externalTargets = append(externalTargets, target)
		}
	}
	return
}

func handleKeys(runInfo *RunInfo, cmdChan chan string) []*upcheck.Target {
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
			upcheck.ShowStatuses(runInfo.checkTargets)
			break
		case 'p':
			if *runInfo.paused {
				fmt.Println("Resuming...")
				*runInfo.paused = false
				go loopCheckAllTargets(runInfo, cmdChan)
			} else {
				fmt.Println("Pausing...")
				*runInfo.paused = true
				cmdChan <- "stop"
			}
		case 'r':
			fmt.Println("Resetting all stats...")
			upcheck.ResetAllStats(runInfo.checkTargets)
		case 'x':
			fmt.Println("Stopping...")
			cmdChan <- "stop"
			fmt.Println("Stopped")
			fmt.Println("Resetting all stats...")
			upcheck.ResetAllStats(runInfo.checkTargets)
			fmt.Println("Reset all stats...")
			upcheck.ShowStatuses(runInfo.checkTargets)
			fmt.Println("Starting...")
			go loopCheckAllTargets(runInfo, cmdChan)
			fmt.Println("Started")
		default:
			fmt.Printf("You pressed: %q\n", char)
		}
	}
	return runInfo.checkTargets
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
