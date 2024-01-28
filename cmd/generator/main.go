package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/manifoldco/promptui"
)

// todo move to flags
var UseCubiomes = true    // generate a random seed
var UseDocker = true      // generate world files
var RavineProximity = 112 // radius
var RavineOffsetNegative = RavineProximity
var RavineOffsetPositive = RavineProximity + 15

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
// todo more context timeout
// todo cubiomes lives after ctrl-c
func main() {
	flag.Parse() // todo move to init?
	defer close(CubiomesInProg)
	defer close(CubiomesDone)
	defer close(CubiomesOut)
	defer close(WorldgenInProg)
	defer close(WorldgenDone)
	defer close(WorldgenDilating)

	if !UseCubiomes {
		log.Printf("info using set seed")
		goto SetSeed
	}

	go Cubiomes()

	log.Printf("info taking it 2 teh next lvl ^_-")
	goto Worldgen

SetSeed:
	// vvv DEBUG SEED vvv
	CubiomesOut <- GodSeed{
		// Seed: "-6916114155717537644",
		Seed: "-448396564840034738",
		Spawn: Coords{
			X: -112,
			Z: -112,
		},
		Shipwreck: Coords{
			X: -80,
			Z: -96,
		},
		Bastion: Coords{
			X: 96,
			Z: -16,
		},
		Fortress: Coords{
			X: -112,
			Z: -96,
		},
	}
	// ^^^ DEBUG SEED ^^^

Worldgen:
	for {
		select {
		case j := <-WorldgenDilating:
			log.Printf("########### WORLDGEN IS DILATING ###########")
			log.Printf("job: %v", j)

			prompt := promptui.Select{
				Label: "Select Action",
				Items: []string{
					"Retry (end of queue)",
					"Go next (save progress)",
					fmt.Sprintf("Quit with %d remaining", *FlagJobs-len(WorldgenDone)),
				},
			}

			promptIndex, _, err := prompt.Run()
			if err != nil {
				log.Fatalf("error prompt failed %v", err)
			}

			switch promptIndex {
			case 0:
				CubiomesOut <- j
			case 1:
				WorldgenDone <- struct{}{}
			case 2:
				fallthrough
			default:
				break Worldgen
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
