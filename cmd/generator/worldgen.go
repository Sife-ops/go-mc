package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	// "time"

	"github.com/docker/docker/api/types"

	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/save"
	"github.com/Tnze/go-mc/save/region"
)

func Worldgen(cubiomesOut chan GodSeed, worldgenInProg chan struct{}, worldgenDone chan struct{}, worldgenDilating chan GodSeed) {
	job := <-cubiomesOut

	// time.Sleep(5 * time.Second)
	// log.Printf("simulating something bad")
	// worldgenDilating <- job
	// return

	log.Printf("info killing old container")
	if err := KillMcContainer(); err != nil {
		if !strings.Contains(err.Error(), "is not running") {
			log.Printf("error %v", err)
			worldgenDilating <- job
			return
		}
	}

	log.Printf("info removing old container")
	if err := RemoveMcContainer(); err != nil {
		log.Printf("error %v", err)
		worldgenDilating <- job
		return
	}

	log.Printf("info deleting previous world folder")
	cmdRmRfWorld := exec.Command("rm", "-rf",
		fmt.Sprintf("%s/tmp/mc/data/world", MustString(os.Getwd())),
	)
	if outRmRfWorld, err := cmdRmRfWorld.Output(); err != nil {
		log.Printf("error deleting world folder: %s %v", string(outRmRfWorld), err)
		worldgenDilating <- job
		return
	}

	log.Printf("info starting minecraft server container")
	mc, _ := ContainerCreateMc(job.Seed)
	if err := DockerClient.ContainerStart(context.TODO(), mc.ID, types.ContainerStartOptions{}); err != nil {
		log.Printf("error starting minecraft server container: %v", err)
		worldgenDilating <- job
		return
	}

	McStarted := make(chan error)
	go AwaitMcStarted(McStarted, mc.ID)

	log.Printf("info waiting for minecraft server to start")
	if err := <-McStarted; err != nil {
		log.Printf("error waiting for minecraft server to start: %v", err)
		worldgenDilating <- job
		return
	}

	McStopped := make(chan error)
	go AwaitMcStopped(McStopped, mc.ID)

	// forceload chunks
	// nether
	ravineAreaX1, ravineAreaZ1, ravineAreaX2, ravineAreaZ2 := job.RavineArea() // todo dont reuse later
	if ec, err := McExec(mc.ID, []string{"rcon-cli",
		fmt.Sprintf(
			"forceload add %d %d %d %d",
			ravineAreaX1, ravineAreaZ1, ravineAreaX2, ravineAreaZ2,
		),
	}); ec != 0 && err != nil {
		log.Printf(
			"error rcon forceloading overworld area %d %d %d %d failed: %v",
			ravineAreaX1, ravineAreaZ1, ravineAreaX2, ravineAreaZ2, err,
		)
		worldgenDilating <- job
		return
	} else {
		log.Printf(
			"info rcon forceloaded overworld area %d %d %d %d",
			ravineAreaX1, ravineAreaZ1, ravineAreaX2, ravineAreaZ2,
		)
	}

	// nether
	forceloadedNetherChunks := []Coords{}
	for _, v := range job.NetherChunksToBastion() {
		forceloadedNetherChunks = append(forceloadedNetherChunks, Coords{v.X, v.Z})
		if ec, err := McExec(mc.ID, []string{"rcon-cli",
			fmt.Sprintf(
				"execute in minecraft:the_nether run forceload add %d %d %d %d",
				v.X*16, v.Z*16, (v.X*16)+15, (v.Z*16)+15,
			),
		}); ec != 0 && err != nil {
			log.Printf(
				"error rcon forceloading nether chunk %d %d failed: %v",
				err, v.X, v.Z,
			)
			worldgenDilating <- job
			return
		}
	}
	log.Printf("info rcon forceloaded nether chunks: %v", forceloadedNetherChunks)

	// stop server
	if ec, err := McExec(mc.ID, []string{"rcon-cli", "stop"}); ec != 0 && err != nil {
		log.Printf("error rcon stopping server: %v", err)
		worldgenDilating <- job
		return
	} else {
		log.Printf("info rcon stopped server")
	}

	log.Printf("info waiting for minecraft server to stop")
	if err := <-McStopped; err != nil {
		log.Printf("error %v", err)
		worldgenDilating <- job
		return
	}

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
			// todo printf job every time u dilate
			log.Printf("error skipping job: %v due to error: %v", job, err)
			worldgenDilating <- job
			return
		}

		for xC := x1 / 16; xC < (x2+1)/16; xC++ {
			for zC := z1 / 16; zC < (z2+1)/16; zC++ {
				// todo check sector exists
				// if !region.ExistSector(0, 0) {
				// 	log.Fatal("sector no existo")
				// }

				data, err := region.ReadSector(ToSector(xC), ToSector(zC))
				if err != nil {
					log.Printf("error %v", err)
					worldgenDilating <- job
					return
				}

				var chunkSave save.Chunk
				err = chunkSave.Load(data)
				if err != nil {
					log.Printf("error %v", err)
					worldgenDilating <- job
					return
				}

				chunkLevel, err := level.ChunkFromSave(&chunkSave)
				if err != nil {
					log.Printf("error %v", err)
					worldgenDilating <- job
					return
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
					log.Printf("error %v", err)
					worldgenDilating <- job
					return
				}
				var chunkSave save.Chunk
				err = chunkSave.Load(data)
				if err != nil {
					log.Printf("error %v", err)
					worldgenDilating <- job
					return
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
		// log.Fatalf("error %v", err)
		log.Printf("warning skipping this seed: %v", err)
		log.Printf("%v", job)
		worldgenDilating <- job
		return
	}

	percentageOfAir := []int{}
	percentageOfAirAvg := 0
	for _, v := range netherChunkCoords {
		// log.Printf("info chunk %d %d", (v.X), (v.Z))
		// log.Printf("info sector %d %d", ToSector(v.X), ToSector(v.Z))
		data, err := region.ReadSector(ToSector(v.X), ToSector(v.Z))
		if err != nil {
			log.Printf("error %v", err)
			worldgenDilating <- job
			return
		}

		var chunkSave save.Chunk
		err = chunkSave.Load(data)
		if err != nil {
			log.Printf("error %v", err)
			worldgenDilating <- job
			return
		}

		chunkLevel, err := level.ChunkFromSave(&chunkSave)
		if err != nil {
			log.Printf("error %v", err)
			worldgenDilating <- job
			return
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

	log.Println()
	log.Printf("info *+*+*+*+* GENERATED GOD SEED *+*+*+*+*")
	log.Printf("info > seed: %s", job.Seed)
	log.Printf("info > magma ravine chunks: %d (%v)", len(magmaRavineChunks), magmaRavineChunks)
	log.Printf("info > shipwrecks with iron: %d (%v)", len(shipwrecksWithIron), shipwrecksWithIron)
	log.Printf("info > pc.s of air toward bastion: %v", percentageOfAir)
	if len(percentageOfAir) > 0 {
		percentageOfAirAvg = percentageOfAirAvg / len(percentageOfAir)
		log.Printf("info > avg. pc. of air toward bastion: %d", percentageOfAirAvg)
	}
	log.Printf("info *+*+*+*+*+*+*+*+*+*+*+*+*+*+*+*+*+*+*")
	log.Println()

	log.Printf("info saving seed")

	// todo do update here, create record in the cubiomes stage
	if _, err := Db.Exec(
		`UPDATE seed SET ravine_chunks=$1, iron_shipwrecks=$2, avg_bastion_air=$3, finished_worldgen=1 WHERE seed=$4`,
		len(magmaRavineChunks), len(shipwrecksWithIron), percentageOfAirAvg, job.Seed,
	); err != nil {
		log.Printf("error %v", err)
		worldgenDilating <- job
		return
	}

	worldgenDone <- struct{}{}
	log.Printf("info finished worldgen job %d", len(worldgenDone))
	<-worldgenInProg
}
