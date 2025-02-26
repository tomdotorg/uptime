package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"upcheck"
)

type RunInfo struct {
	mu                 sync.RWMutex
	programStartedTime *time.Time
	configTime         time.Time // when this config was loaded
	checkTargets       []*upcheck.Target
	networkInfo        *upcheck.NetworkInfo
	configFilename     *string
	interval           *int
	paused             *bool
}

const CONFIGFILE = "hosts.txt"

func main() {
	runInfo := RunInfo{
		programStartedTime: func() *time.Time { t := time.Now(); return &t }(),
		configFilename:     flag.String("f", CONFIGFILE, "Filename containing the targets"),
		interval:           flag.Int("i", 2, "Number of seconds between each check"),
		paused:             new(bool), // zero value is false
	}

	// Parse the command line flags
	flag.Parse()
	initLogs()

	// Initialize keyboard listener
	if err := keyboard.Open(); err != nil {
		log.Fatal().Err(err).Msg("Failed to open keyboard")
	}
	defer func() {
		if err := keyboard.Close(); err != nil {
			log.Fatal().Err(err).Msg("Failed to close keyboard")
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())

	runInfo.checkTargets = upcheck.LoadTargets(ctx, *runInfo.configFilename)
	runInfo.configTime = time.Now()

	// to here, we are in one goroutine.
	checkChan := make(chan upcheck.CheckInfo)

	go listenForCheckInfo(ctx, &runInfo, checkChan)
	go watchForNetworkChanges(ctx, &runInfo, checkChan)

	fmt.Println("Checking all targets (s key for status, ? for help)...")
	// fire off a goroutine for each target
	checkAllTargets(ctx, runInfo.checkTargets, *runInfo.interval, checkChan)

	for keepGoing := true; keepGoing; {
		keepGoing = handleKeys(&runInfo)
	}
	log.Info().Msg("keepGoing is false. calling cancel()")
	close(checkChan)
	cancel()
}

func checkAllTargets(ctx context.Context, targets []*upcheck.Target, interval int, checkChan chan<- upcheck.CheckInfo) {
	for _, target := range targets {
		go upcheck.PeriodicallyCheckHost(target.Host, target.Port, interval, ctx, target.CmdChan, checkChan)
	}
}

func watchForNetworkChanges(ctx context.Context, runInfo *RunInfo, checkChan chan<- upcheck.CheckInfo) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Context canceled, stopping watchForNetworkChanges()")
			return
		case <-ticker.C:
			newNetInfo, err := upcheck.GetNetworkInfo()
			if err != nil { // network is down
				log.Warn().Msg("Error getting network info - must be down")
				runInfo.mu.Lock()
				runInfo.networkInfo = nil
				runInfo.mu.Unlock()
			} else { // network is valid
				// if it has changed, make sure we have the default gw in the target list
				if !newNetInfo.Equals(runInfo.networkInfo) {
					runInfo.mu.Lock()
					runInfo.networkInfo = newNetInfo
					runInfo.mu.Unlock()
					log.Info().Msgf("Network changed.\n%v", newNetInfo)
					results, gw := upcheck.AddDefaultGatewayTarget(ctx, runInfo.checkTargets, runInfo.networkInfo)
					if gw != nil {
						log.Info().Msgf("added default gw: %v", gw)
						runInfo.mu.Lock()
						runInfo.checkTargets = results
						runInfo.mu.Unlock()
						go upcheck.PeriodicallyCheckHost(gw.Host, gw.Port, *runInfo.interval, ctx, gw.CmdChan, checkChan)
					}
				}
			}
		}
	}
}

/*
listenForCheckInfo listens for incoming CheckInfo data from a channel and updates the corresponding
Target in the RunInfo's checkTargets slice. It logs the receipt of valid CheckInfo and updates the
target's status. If the CheckInfo corresponds to an unknown target, a warning is logged.

Parameters:
- runInfo: A pointer to RunInfo containing the application's runtime information and target list.
- checks: A receive-only channel of CheckInfo from which the function reads check information.

The function uses synchronization mechanisms to ensure thread-safe updates to the target data.
*/
func listenForCheckInfo(ctx context.Context, runInfo *RunInfo, checks <-chan upcheck.CheckInfo) {
	log.Info().Msg("Listening for checkInfo")
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Context canceled, stopping listenForCheckInfo()")
			return
		case checkInfo := <-checks:
			log.Debug().Msgf("Received checkInfo: %v", checkInfo)
			runInfo.mu.RLock()
			target := upcheck.FindTarget(runInfo.checkTargets, checkInfo.Host, checkInfo.Port)
			runInfo.mu.RUnlock()
			if target != nil {
				log.Debug().Msgf("Received valid checkInfo for target: %s:%d", target.Host, target.Port)
				target.Update(checkInfo)
				target.Mu.RLock()
				log.Debug().Msgf("%s:%d updated.", target.Host, target.Port)
				target.Mu.RUnlock()
			} else {
				log.Warn().Msgf("Received checkInfo (%v) for unknown target: %s:%d", checkInfo, checkInfo.Host, checkInfo.Port)
			}
		}
	}
}

func showTargets(runInfo *RunInfo) {
	fmt.Println("\n" + time.Now().Format("2006-01-02 15:04:05"))
	runInfo.mu.RLock()
	defer runInfo.mu.RUnlock()
	fmt.Println("Up: ", time.Since(*runInfo.programStartedTime).Round(time.Second))
	if runInfo.networkInfo == nil {
		fmt.Println("No network info available")
	} else {
		fmt.Printf("\nNetwork Config:\n%v\n\n", runInfo.networkInfo)
	}
	subnetTargets, gatewayTargets, externalTargets, ok := upcheck.ClassifyTargets(runInfo.checkTargets, &upcheck.NetworkInfo{Address: runInfo.networkInfo.Address, Mask: runInfo.networkInfo.Mask, GW: runInfo.networkInfo.GW})
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

func handleKeys(runInfo *RunInfo) bool {
	if char, key, err := keyboard.GetKey(); err == nil {
		if key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
			fmt.Println("Exiting...")
			return false
		}
		switch char {
		case 'q', 'x':
			fmt.Println("Exiting...")
			return false
		case 's':
			showTargets(runInfo)
		case 'p':
			runInfo.mu.Lock()
			if *runInfo.paused {
				fmt.Println("Resuming...")
				*runInfo.paused = false
				resumeAllTargets(runInfo.checkTargets)
			} else {
				fmt.Println("Pausing...")
				*runInfo.paused = true
				pauseAllTargets(runInfo.checkTargets)
			}
			runInfo.mu.Unlock()
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

func pauseAllTargets(targets []*upcheck.Target) {
	for _, target := range targets {
		target.CmdChan <- upcheck.PAUSE
	}
}

func resumeAllTargets(targets []*upcheck.Target) {
	for _, target := range targets {
		target.CmdChan <- upcheck.RESUME
	}
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
