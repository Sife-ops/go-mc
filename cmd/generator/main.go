package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"
)

var RavineProximity = 4 // chunk radius
var RavineOffsetNegative = RavineProximity * 16
var RavineOffsetPositive = RavineOffsetNegative + 15

var FlagThreads = flag.Int("t", 2, "threads")
var FlagJobs = flag.Int("j", 2, "jobs")

var CubiomesCtx, CancelCubiomesCtx = context.WithCancel(context.Background())
var CubiomesDone chan struct{}
var CubiomesOut chan GodSeed

var WorldgenCtx, CancelWorldgenCtx = context.WithCancel(context.Background())
var WorldgenDone chan struct{}
var WorldgenRecovering = make(chan GodSeed)

func init() {
	flag.Parse()
	CubiomesDone = make(chan struct{}, *FlagJobs)
	CubiomesOut = make(chan GodSeed, *FlagJobs)
	WorldgenDone = make(chan struct{}, *FlagJobs)
}

// todo html
// todo ability to load unfinished seeds
func main() {
	defer CancelCubiomesCtx()
	defer close(CubiomesDone)
	defer close(CubiomesOut)
	defer CancelWorldgenCtx()
	defer close(WorldgenDone)
	defer close(WorldgenRecovering)

	go func() {
		sigchan := make(chan os.Signal)
		signal.Notify(sigchan, os.Interrupt)
		<-sigchan
		log.Printf("info cleaning up")
		CancelCubiomesCtx()
		CancelWorldgenCtx()
		os.Exit(0)
	}()

	for i := 0; i < *FlagThreads; i++ {
		go Cubiomes(CubiomesCtx, CancelCubiomesCtx)
	}

Worldgen:
	for {
		select {
		case <-WorldgenCtx.Done():
			return

		case j := <-WorldgenRecovering:
			log.Printf("########### WORLDGEN IS RECOVERING ###########")
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
			case <-time.After(30 * time.Second):
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

		default:
			Worldgen(WorldgenCtx, CancelWorldgenCtx)
		}
	}
}
