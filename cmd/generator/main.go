package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"

	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/save"
	"github.com/Tnze/go-mc/save/region"
)

// todo move to flags
var UseCubiomes = true    // generate a random seed
var UseDocker = true      // generate world files
var RavineProximity = 112 // radius
var RavineOffsetNegative = RavineProximity
var RavineOffsetPositive = RavineProximity + 15

// todo html
// todo get rid of some fatals
// todo docker service api 4 golang?
func main() {
	flagThreads := flag.Int("t", 2, "threads")
	flagJobs := flag.Int("j", 2, "jobs")
	flag.Parse()

	var JobsInProgress = make(chan struct{}, *flagThreads)
	var JobsDone = make(chan struct{}, *flagJobs)
	var Queue1 = make(chan GodSeed, *flagJobs)

	if !UseCubiomes {
		log.Printf("info using set seed")
		goto SetSeed
	}

	go func(jip chan struct{}, jd chan struct{}, q1 chan GodSeed) {
		for len(jd) < *flagJobs {
			// log.Printf("info %d cubiomes instances running", len(jip))
			// log.Printf("info %d cubiomes jobs done", len(jd))
			// log.Printf("info %d items in q1", len(q1))
			time.Sleep(1 * time.Second)
		}
		log.Printf("info ==&==&==&==&==&==&==")
		log.Printf("info shutting down channels")
		close(jip)
		close(jd)
		close(q1)
	}(JobsInProgress, JobsDone, Queue1)

	for t := 0; t < *flagJobs; t++ {
		go func(jip chan struct{}, jd chan struct{}, q1 chan GodSeed) {
		Wait:
			for len(jip) >= *flagThreads {
				time.Sleep(660 * time.Millisecond)
			}
			if len(jip) < *flagThreads {
				jip <- struct{}{}
			} else {
				goto Wait
			}

			cmdCubiomes := exec.Command("./a.out")
			outCubiomes, err := cmdCubiomes.Output()
			if err != nil {
				log.Fatalf("error %v", err)
			}
			log.Printf("info cubiomes output: %s", string(outCubiomes))

			outCubiomesArr := strings.Split(string(outCubiomes), ":")
			q1 <- GodSeed{
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
			<-jip
			jd <- struct{}{}
		}(JobsInProgress, JobsDone, Queue1)
	}

	log.Printf("info taking it 2 teh next lvl ^_-")
	goto Phaze2

SetSeed:
	// vvv DEBUG SEED vvv
	Queue1 <- GodSeed{
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
	close(JobsInProgress)
	close(JobsDone)
	close(Queue1)
	// ^^^ DEBUG SEED ^^^

Phaze2:
	for iJob := 0; iJob < *flagJobs; iJob++ {
		job := <-Queue1

		ravineAreaX1, ravineAreaZ1, ravineAreaX2, ravineAreaZ2 := job.RavineArea()
		if !UseDocker {
			log.Printf("info skipping docker")
			goto SkipDocker
		}
		{
			log.Printf("info removing old container")
			if err := RemoveMcContainer(); err != nil {
				log.Fatalf("error %v", err)
			}

			log.Printf("info deleting previous world folder")
			cmdRmRfWorld := exec.Command("rm", "-rf",
				fmt.Sprintf("%s/tmp/mc/data/world", MustString(os.Getwd())),
			)
			if outRmRfWorld, err := cmdRmRfWorld.Output(); err != nil {
				log.Printf("error delete world output: %s", string(outRmRfWorld))
				log.Fatalf("error %v", err)
			}

			mc, _ := ContainerCreateMc(job.Seed)
			if err := DockerClient.ContainerStart(context.TODO(), mc.ID, types.ContainerStartOptions{}); err != nil {
				log.Fatalf("error %v", err)
			}

			McStopped := make(chan error)
			go AwaitMcStopped(McStopped, mc.ID)

			McStarted := make(chan error)
			go AwaitMcStarted(McStarted, mc.ID)

			log.Printf("info waiting for minecraft server to start")
			if err := <-McStarted; err != nil {
				log.Fatalf("error %v", err)
			}

			// forceload chunks
			//overworld
			if ec, err := McExec(mc.ID, []string{"rcon-cli",
				fmt.Sprintf(
					"forceload add %d %d %d %d",
					ravineAreaX1, ravineAreaZ1, ravineAreaX2, ravineAreaZ2,
				),
			}); err != nil {
				log.Fatalf("error %v", err)
			} else {
				log.Printf("info rcon forceload overworld chunks: %d", ec)
			}

			for _, v := range job.NetherChunksToBastion() {
				if ec, err := McExec(mc.ID, []string{"rcon-cli",
					fmt.Sprintf(
						"execute in minecraft:the_nether run forceload add %d %d %d %d",
						v.X*16, v.Z*16, (v.X*16)+15, (v.Z*16)+15,
					),
				}); err != nil {
					log.Fatalf("error %v", err)
				} else {
					log.Printf("info rcon forceload nether chunks: %d", ec)
				}
			}

			// stop server
			if ec, err := McExec(mc.ID, []string{"rcon-cli", "stop"}); err != nil {
				log.Fatalf("error %v", err)
			} else {
				log.Printf("info rcon stop: %d", ec)
			}

			log.Printf("info waiting for minecraft server to stop")
			if err := <-McStopped; err != nil {
				log.Fatalf("error %v", err)
			}
		}
	SkipDocker:

		// overworld checks
		// this part is RLLY bad
		shipwreckAreaX1, shipwreckAreaZ1, shipwreckAreaX2, shipwreckAreaZ2 := job.ShipwreckArea()
		magmaRavineChunks := []Coords{}
		shipwrecksWithIron := []string{}
		for quadrant := 0; quadrant < 4; quadrant++ {
			// todo swap x/z
			regionX := (quadrant % 2) - 1
			regionZ := (quadrant / 2) - 1

			x1 := ravineAreaX1
			z1 := ravineAreaZ1
			x2 := ravineAreaX2
			z2 := ravineAreaZ2
			if regionX < 0 {
				if x1 >= 0 {
					log.Printf("info no overlap with -X regions")
					goto NextQuadrant
				}
				if x2 >= 0 {
					x2 = -1
				}
			} else {
				if x2 < 0 {
					log.Printf("info no overlap with +X regions")
					goto NextQuadrant
				}
				if x1 < 0 {
					x1 = 0
				}
			}
			if regionZ < 0 {
				if z1 >= 0 {
					log.Printf("info no overlap with -Z regions")
					goto NextQuadrant
				}
				if z2 >= 0 {
					z2 = -1
				}
			} else {
				if z2 < 0 {
					log.Printf("info no overlap with +Z regions")
					goto NextQuadrant
				}
				if z1 < 0 {
					z1 = 0
				}
			}
			goto OpenRegion

		NextQuadrant:
			log.Printf(
				"info skipping region %d,%d due to no overlap with ravine area around shipwreck %d %d %d %d",
				regionX, regionZ, ravineAreaX1, ravineAreaZ1, ravineAreaX2, ravineAreaZ2,
			)
			continue

		OpenRegion:
			region, err := region.Open(fmt.Sprintf(
				"%s/tmp/mc/data/world/region/r.%d.%d.mca",
				MustString(os.Getwd()), regionX, regionZ,
			))
			if err != nil {
				log.Fatalf("error %v", err)
			}

			for xC := x1 / 16; xC < (x2+1)/16; xC++ {
				for zC := z1 / 16; zC < (z2+1)/16; zC++ {
					// todo check sector exists
					// if !region.ExistSector(0, 0) {
					// 	log.Fatal("sector no existo")
					// }

					data, err := region.ReadSector(ToSector(xC), ToSector(zC))
					if err != nil {
						log.Fatalf("error %v", err)
					}

					var chunkSave save.Chunk
					err = chunkSave.Load(data)
					if err != nil {
						log.Fatalf("error %v", err)
					}

					chunkLevel, err := level.ChunkFromSave(&chunkSave)
					if err != nil {
						log.Fatalf("error %v", err)
						// log.Printf("warning skipping seed %s due to error: %v", job.Seed, err)
						// continue Phaze2
					}

					obby, magma, lava := 0, 0, 0
					y10 := 256 * 10
					for i := y10; i < y10+256; i++ {
						x := chunkLevel.GetBlockID(1, i)
						if x == "minecraft:magma_block" {
							magma++
						}
						if x == "minecraft:obsidian" {
							obby++
						}
					}
					y9 := 256 * 9
					for i := y9; i < y9+256; i++ {
						x := chunkLevel.GetBlockID(1, i)
						if x == "minecraft:lava" {
							lava++
						}
					}
					// todo move to flags
					if obby >= 30 && magma >= 10 && lava >= 30 {
						magmaRavineChunks = append(magmaRavineChunks, Coords{xC, zC})
					}
				}
			}

			x1 = shipwreckAreaX1
			z1 = shipwreckAreaZ1
			x2 = shipwreckAreaX2
			z2 = shipwreckAreaZ2
			if regionX < 0 {
				if x1 >= 0 {
					goto CloseRegion
				}
				if x2 >= 0 {
					x2 = -1
				}
			} else {
				if x2 < 0 {
					goto CloseRegion
				}
				if x1 < 0 {
					x1 = 0
				}
			}
			if regionZ < 0 {
				if z1 >= 0 {
					goto CloseRegion
				}
				if z2 >= 0 {
					z2 = -1
				}
			} else {
				if z2 < 0 {
					goto CloseRegion
				}
				if z1 < 0 {
					z1 = 0
				}
			}

			for xC := x1 / 16; xC < (x2+1)/16; xC++ {
				for zC := z1 / 16; zC < (z2+1)/16; zC++ {
					data, err := region.ReadSector(ToSector(xC), ToSector(zC))
					if err != nil {
						log.Fatalf("error %v", err)
					}
					var chunkSave save.Chunk
					err = chunkSave.Load(data)
					if err != nil {
						log.Fatalf("error %v", err)
					}
					if len(chunkSave.Level.Structures.Starts.Shipwreck.Children) < 1 {
						continue
					}
					for _, v := range chunkSave.Level.Structures.Starts.Shipwreck.Children {
						if v.Template == "minecraft:shipwreck/rightsideup_backhalf" ||
							v.Template == "minecraft:shipwreck/rightsideup_backhalf_degraded" ||
							v.Template == "minecraft:shipwreck/rightsideup_full" ||
							v.Template == "minecraft:shipwreck/rightsideup_full_degraded" ||
							v.Template == "minecraft:shipwreck/sideways_backhalf" ||
							v.Template == "minecraft:shipwreck/sideways_backhalf_degraded" ||
							v.Template == "minecraft:shipwreck/sideways_full" ||
							v.Template == "minecraft:shipwreck/sideways_full_degraded" ||
							v.Template == "minecraft:shipwreck/upsidedown_backhalf" ||
							v.Template == "minecraft:shipwreck/upsidedown_backhalf_degraded" ||
							v.Template == "minecraft:shipwreck/upsidedown_full" ||
							v.Template == "minecraft:shipwreck/upsidedown_full_degraded" ||
							v.Template == "minecraft:shipwreck/with_mast" ||
							v.Template == "minecraft:shipwreck/with_mast_degraded" {
							shipwrecksWithIron = append(shipwrecksWithIron, v.Template)
						}
					}
				}
			}

		CloseRegion:
			// todo use defer?
			region.Close()
		}

		// nether checks
		// todo ckeck for lava lake
		netherChunkCoords := job.NetherChunksToBastion()

		region, err := region.Open(fmt.Sprintf(
			"%s/tmp/mc/data/world/DIM-1/region/r.%d.%d.mca",
			MustString(os.Getwd()), netherChunkCoords[0].X, netherChunkCoords[0].Z,
		))
		if err != nil {
			log.Fatalf("error %v", err)
		}

		percentageOfAir := []int{}
		percentageOfAirAvg := 0
		for _, v := range netherChunkCoords {
			// log.Printf("info chunk %d %d", (v.X), (v.Z))
			// log.Printf("info sector %d %d", ToSector(v.X), ToSector(v.Z))
			data, err := region.ReadSector(ToSector(v.X), ToSector(v.Z))
			if err != nil {
				log.Fatalf("error %v", err)
			}

			var chunkSave save.Chunk
			err = chunkSave.Load(data)
			if err != nil {
				log.Fatalf("error %v", err)
			}

			chunkLevel, err := level.ChunkFromSave(&chunkSave)
			if err != nil {
				log.Printf("warning skipping seed %s due to error: %v", job.Seed, err)
				// goto NextQueueItem
				continue Phaze2
			}

			airBlocks := 0
			for i := 1; i < 9; i++ {
				for j := 0; j < 16*16*16; j++ {
					x := chunkLevel.GetBlockID(i, j)
					if x == "minecraft:air" {
						airBlocks++
					}
				}
			}

			percentageOfAirChunk := int((float64(airBlocks) * 100) / 32768)
			percentageOfAir = append(percentageOfAir, percentageOfAirChunk)
			percentageOfAirAvg += percentageOfAirChunk
		}

		log.Printf("info pc.s of air toward bastion: %v", percentageOfAir)
		if len(percentageOfAir) > 0 {
			percentageOfAirAvg = percentageOfAirAvg / len(percentageOfAir)
			log.Printf("info avg. pc. of air toward bastion: %d", percentageOfAirAvg)
		}

		log.Printf("info *+*+*+*+*+*+*+*+*+*+")
		log.Printf("info > seed: %s", job.Seed)
		log.Printf("info > magma ravine chunks: %d (%v)", len(magmaRavineChunks), magmaRavineChunks)
		log.Printf("info > shipwrecks with iron: %d (%v)", len(shipwrecksWithIron), shipwrecksWithIron)
		log.Printf("info saving seed")

		if _, err := Db.Exec(
			"INSERT INTO seed (seed, ravine_chunks, iron_shipwrecks, avg_bastion_air) VALUES ($1, $2, $3, $4)",
			job.Seed, len(magmaRavineChunks), len(shipwrecksWithIron), percentageOfAirAvg,
		); err != nil {
			log.Fatalf("error %v", err)
		}
	}
}
