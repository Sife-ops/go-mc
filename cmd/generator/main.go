package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
)

// todo move to flags
var UseCubiomes = true    // generate a random seed
var UseDocker = true      // generate world files
var RavineProximity = 112 // radius
var RavineOffsetNegative = RavineProximity
var RavineOffsetPositive = RavineProximity + 15

// todo html
// todo more context timeout
// todo cubiomes lives after ctrl-c
func main() {
	flagThreads := flag.Int("t", 2, "threads")
	flagJobs := flag.Int("j", 2, "jobs")
	flag.Parse()

	var CubiomesInProg = make(chan struct{}, *flagThreads)
	defer close(CubiomesInProg)
	var CubiomesDone = make(chan struct{}, *flagJobs)
	defer close(CubiomesDone)
	var CubiomesOut = make(chan GodSeed, *flagJobs)
	defer close(CubiomesOut)

	var WorldgenInProg = make(chan struct{}, 1)
	defer close(WorldgenInProg)
	var WorldgenDone = make(chan struct{}, *flagJobs)
	defer close(WorldgenDone)
	var WorldgenDilating = make(chan GodSeed)
	defer close(WorldgenDilating)

	if !UseCubiomes {
		log.Printf("info using set seed")
		goto SetSeed
	}

	go func() {
		for {
			<-time.After(100 * time.Millisecond)

			if len(CubiomesDone) >= *flagJobs {
				return
			}
			todo := *flagJobs - len(CubiomesDone)
			if todo >= *flagThreads {
				if len(CubiomesInProg) >= *flagThreads {
					continue
				}
			} else {
				if len(CubiomesInProg) >= todo {
					continue
				}
			}

			CubiomesInProg <- struct{}{}
			go func() {
				log.Println("info starting new cubiomes thread")
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				cmdCubiomes := exec.CommandContext(ctx, "./a.out")
				outCubiomes, err := cmdCubiomes.Output()
				outCubiomesArr := strings.Split(string(outCubiomes), ":")
				if err != nil {
					log.Printf("warning cubiomes job exited %v", err)
					goto CubiomesExit
				}
				log.Printf("info cubiomes output: %s", string(outCubiomes))

				{
					gs := GodSeed{
						Seed: outCubiomesArr[0],
						Spawn: Coords{
							X: MustInt(strconv.Atoi(strings.Split(outCubiomesArr[1], ",")[0])),
							Z: MustInt(strconv.Atoi(strings.Split(outCubiomesArr[1], ",")[1])),
						},
						Shipwreck: Coords{
							X: MustInt(strconv.Atoi(strings.Split(outCubiomesArr[2], ",")[0])),
							Z: MustInt(strconv.Atoi(strings.Split(outCubiomesArr[2], ",")[1])),
						},
						Bastion: Coords{
							X: MustInt(strconv.Atoi(strings.Split(outCubiomesArr[3], ",")[0])),
							Z: MustInt(strconv.Atoi(strings.Split(outCubiomesArr[3], ",")[1])),
						},
						Fortress: Coords{
							X: MustInt(strconv.Atoi(strings.Split(outCubiomesArr[4], ",")[0])),
							Z: MustInt(strconv.Atoi(strings.Split(outCubiomesArr[4], ",")[1])),
						},
					}

					if _, err := Db.Exec(
						`INSERT INTO seed (seed, spawn_x, spawn_z, bastion_x, bastion_z, shipwreck_x, shipwreck_z, fortress_x, fortress_z, finished_cubiomes) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1)`,
						gs.Seed, gs.Spawn.X, gs.Spawn.Z, gs.Bastion.X, gs.Bastion.Z, gs.Shipwreck.X, gs.Shipwreck.Z, gs.Fortress.X, gs.Fortress.Z,
					); err != nil {
						log.Fatalf("error %v", err)
					}

					CubiomesOut <- gs
				}

				CubiomesDone <- struct{}{}
				log.Printf("info finished cubiomes job %d", len(CubiomesDone))

			CubiomesExit:
				log.Println("info freeing cubiomes thread")
				cancel()
				<-CubiomesInProg
			}()
		}
	}()

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
					fmt.Sprintf("Quit with %d remaining", *flagJobs-len(WorldgenDone)),
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
			if len(WorldgenDone) >= *flagJobs {
				break Worldgen
			}
			if len(WorldgenInProg) >= 1 {
				continue
			}

			WorldgenInProg <- struct{}{}
			go Worldgen(CubiomesOut, WorldgenInProg, WorldgenDone, WorldgenDilating)
		}
	}
}
