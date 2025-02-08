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

type RunInfo struct {
	programStartedTime *time.Time
	networkInfo        *upcheck.NetworkInfo
	configTime         *time.Time
	checkTargets       []*upcheck.Target
	configFilename     *string
	interval           *int
	paused             *bool
	// app                *tview.Application
}

const CONFIGFILE = "hosts.txt"

func main() {
	runInfo := RunInfo{
		programStartedTime: func() *time.Time { t := time.Now(); return &t }(),
		// app:                tview.NewApplication(),
		configFilename: flag.String("f", CONFIGFILE, "Filename containing the targets"),
		interval:       flag.Int("i", 2, "Number of seconds between target checks"),
		paused:         new(bool), // zero value is false
	}

	// Parse the command line flags
	flag.Parse()
	initLogs()

	if netInfo, err := upcheck.GetNetworkInfo(); err != nil {
		log.Fatal().Err(err).Msg("Error getting network info - exiting")
	} else {
		runInfo.networkInfo = &netInfo
		fmt.Printf("Network Info:\n%v\n", netInfo)
	}

	runInfo.checkTargets = upcheck.LoadTargets(*runInfo.configFilename)

	// Initialize keyboard listener
	if err := keyboard.Open(); err != nil {
		log.Fatal().Err(err).Msg("Failed to open keyboard")
	}
	defer func() {
		if err := keyboard.Close(); err != nil {
			log.Fatal().Err(err).Msg("Failed to close keyboard")
		}
	}()
	fmt.Println("Checking all targets (s key for status, ? for help)...")

	cmdChan := make(chan string)
	go loopCheckAllTargets(&runInfo, cmdChan)
	for keepGoing := handleKeys(&runInfo, cmdChan); keepGoing; {
		keepGoing = handleKeys(&runInfo, cmdChan)
	}
}

func showTargets(runInfo RunInfo) {
	fmt.Println("\n" + time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("Up: ", time.Since(*runInfo.programStartedTime).Round(time.Second))
	if runInfo.networkInfo == nil {
		fmt.Println("No network info available")
	} else {
		fmt.Printf("\nNetwork Config:\n%v\n\n", *runInfo.networkInfo)
	}
	subnetTargets, gatewayTargets, externalTargets, ok := upcheck.ClassifyTargets(runInfo.checkTargets, runInfo.networkInfo)
	if ok {
		fmt.Println("")
		ShowStatuses("Subnet Targets", subnetTargets)
		fmt.Println("")
		ShowStatuses("Gateway Targets", gatewayTargets)
		fmt.Println("")
		ShowStatuses("External Targets", externalTargets)
	} else {
		ShowStatuses("All Targets", runInfo.checkTargets)
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
			newNetInfo, err := upcheck.GetNetworkInfo()
			if err != nil {
				if runInfo.networkInfo != nil {
					runInfo.networkInfo = nil
					log.Warn().Msg("No network connection detected - skipping checks")
					upcheck.TargetsOffline(runInfo.checkTargets)
				}
			} else {
				if runInfo.networkInfo == nil || !runInfo.networkInfo.Equals(newNetInfo) {
					log.Warn().Msg("Network change detected")
					runInfo.networkInfo = &newNetInfo
					fmt.Printf("Network info:\n%v\n", newNetInfo)
					log.Info().Msg("Ensuring default gateway is in targets")
					runInfo.checkTargets = upcheck.AddDefaultGatewayTarget(runInfo.checkTargets, runInfo.networkInfo)
					showTargets(*runInfo)
				}
				upcheck.CheckAllTargets(runInfo.checkTargets)
			}
		}
	}
}

func handleKeys(runInfo *RunInfo, cmdChan chan string) bool {
	// Check for key presses
	if char, key, err := keyboard.GetKey(); err == nil {
		if key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			fmt.Println("Exiting...")
			cmdChan <- "stop"
			return false
		}
		switch char {
		case 'q', 'x':
			fmt.Println("Exiting...")
			return false
		case 's':
			showTargets(*runInfo)
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
			fmt.Println("(s key for status, ? for help)...")
		}
	}
	return true
}

func showHelp() {
	fmt.Println("\nupcheck - a simple network uptime checker")
	fmt.Println("flags:" +
		"\n  -f <filename> : Filename containing the targets" +
		"\n  -i <interval> : Number of seconds between target checks")
	fmt.Println("Commands:")
	fmt.Println("  q or x: quit")
	fmt.Println("  s: show all targets")
	fmt.Println("  p: pause/resume checking")
	fmt.Println("  r: reset all stats")
	fmt.Println("\ninput file format (one per line):\n8.8.4.4:53\ngoogle.com:80\n192.168.1.1:80")
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

func ShowStatus(target *upcheck.Target) string {
	return fmt.Sprintf("%+v", target.String())
}

func ShowStatuses(heading string, targets []*upcheck.Target) {
	fmt.Println(heading)
	fmt.Println("")
	for _, target := range targets {
		fmt.Println(ShowStatus(target))
	}
}
