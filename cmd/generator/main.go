package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"time"
)

var RavineProximity = 4 // chunk radius
var RavineOffsetNegative = RavineProximity * 16
var RavineOffsetPositive = RavineOffsetNegative + 15

var FlagThreads = flag.Int("t", 2, "threads")
var FlagJobs = flag.Int("j", 2, "jobs")

var CubiomesInProg chan struct{}
var CubiomesDone chan struct{}
var CubiomesOut chan GodSeed

var WorldgenInProg = make(chan struct{}, 1)
var WorldgenDone chan struct{}
var WorldgenDilating = make(chan GodSeed)

func init() {
	flag.Parse()
	CubiomesInProg = make(chan struct{}, *FlagThreads)
	CubiomesDone = make(chan struct{}, *FlagJobs)
	CubiomesOut = make(chan GodSeed, *FlagJobs)
	WorldgenDone = make(chan struct{}, *FlagJobs)
}

// todo html
// todo ability to load unfinished seeds
func main() {
	go func() {
		sigchan := make(chan os.Signal)
		signal.Notify(sigchan, os.Interrupt)
		<-sigchan
		log.Printf("info kill cubiomes processes")
		exec.Command("pkill", "cubiomes").Output()
		os.Exit(0)
	}()

	flag.Parse() // todo move to init?
	defer close(CubiomesInProg)
	defer close(CubiomesDone)
	defer close(CubiomesOut)
	defer close(WorldgenInProg)
	defer close(WorldgenDone)
	defer close(WorldgenDilating)

	go Cubiomes()

Worldgen:
	for {
		select {
		case j := <-WorldgenDilating:
			log.Printf("########### WORLDGEN IS DILATING ###########")
			log.Printf("job: %v", j)

			PromptIndex := make(chan int)
			go func() {
				fmt.Printf(">>> select action\n")
				fmt.Printf(">>> 1) go next (save progress) (default)\n")
				fmt.Printf(">>> 2) add to end of queue\n")
				fmt.Printf(">>> 3) quit with %d worldgen jobs remaining\n", *FlagJobs-len(WorldgenDone))
				var action string
				fmt.Scanln(&action)
				actionInt, err := strconv.Atoi(action)
				if err != nil || actionInt < 1 || actionInt > 3 {
					PromptIndex <- 0
					return
				}
				PromptIndex <- actionInt - 1
			}()

			select {
			case <-time.After(10 * time.Second):
				log.Printf("info progress saved")
				WorldgenDone <- struct{}{}
			case promptIndex := <-PromptIndex:
				switch promptIndex {
				case 0:
					log.Printf("info progress saved")
					WorldgenDone <- struct{}{}
				case 1:
					log.Printf("info added to queue")
					CubiomesOut <- j
				case 2:
					fallthrough
				default:
					log.Printf("info exiting")
					break Worldgen
				}
			}

			<-WorldgenInProg

		case <-time.After(100 * time.Millisecond):
			if len(WorldgenDone) >= *FlagJobs {
				break Worldgen
			}
			if len(WorldgenInProg) >= 1 {
				continue
			}

			WorldgenInProg <- struct{}{}
			go Worldgen()
		}
	}
}
