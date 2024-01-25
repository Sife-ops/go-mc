package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/level/block"
	"github.com/Tnze/go-mc/save"
	"github.com/Tnze/go-mc/save/region"
)

var UseCubiomes = false   // generate a random seed
var UseSeedFile = true    // write cubiomes output to file
var UseDocker = true      // write cubiomes output to file
var RavineProximity = 240 // diameter
var RavineOffsetNegative = (RavineProximity - 16) / 2
var RavineOffsetPositive = RavineOffsetNegative + 15

//go:embed template
var fsTemplate embed.FS

type Coords struct {
	X int
	Z int
}

type GodSeed struct {
	Seed      string // todo mb use int?
	Spawn     Coords
	Shipwreck Coords
}

func (g *GodSeed) RavineArea() (int, int, int, int) {
	return g.Shipwreck.X - RavineOffsetNegative,
		g.Shipwreck.Z - RavineOffsetNegative,
		g.Shipwreck.X + RavineOffsetPositive,
		g.Shipwreck.Z + RavineOffsetPositive
}

func (g *GodSeed) ShipwreckArea() (int, int, int, int) {
	return g.Shipwreck.X - 16,
		g.Shipwreck.Z - 16,
		g.Shipwreck.X + 31,
		g.Shipwreck.Z + 31
}

func mustInt(i int, err error) int {
	if err != nil {
		panic(err)
	}
	return i
}

func toRegion(i int) int {
	if i < 0 {
		return -1
	}
	return 0
}

func toSector(i int) int {
	if i < 0 {
		return 32 + i
	}
	return i
}

func main() {
	godSeed := GodSeed{
		Seed: "-6916114155717537644",
		Spawn: Coords{
			X: 0,
			Z: 16,
		},
		Shipwreck: Coords{
			X: 16,
			Z: 64,
		},
	}

	{
		if !UseCubiomes {
			goto SkipCubiomes
		}
		cmdCubiomes := exec.Command("./a.out")
		outCubiomes, err := cmdCubiomes.Output()
		if err != nil {
			log.Fatal(err)
		}
		log.Println(string(outCubiomes))
		outCubiomesArr := strings.Split(string(outCubiomes), ":")
		godSeed = GodSeed{
			Seed: outCubiomesArr[0],
			Spawn: Coords{
				X: mustInt(strconv.Atoi(strings.Split(outCubiomesArr[1], ",")[0])),
				Z: mustInt(strconv.Atoi(strings.Split(outCubiomesArr[1], ",")[1])),
			},
			Shipwreck: Coords{
				X: mustInt(strconv.Atoi(strings.Split(outCubiomesArr[2], ",")[0])),
				Z: mustInt(strconv.Atoi(strings.Split(outCubiomesArr[2], ",")[1])),
			},
		}
	}
SkipCubiomes:

	{
		if !UseSeedFile {
			goto SkipSeedFile
		}
		fileSeedOut, err := os.Create("./seed.txt")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(fileSeedOut, "seed: %s\n", godSeed.Seed)
		fmt.Fprintf(fileSeedOut, "spawn: %d, %d\n", godSeed.Spawn.X, godSeed.Spawn.Z)
		fmt.Fprintf(fileSeedOut, "shipwreck: %d, %d\n", godSeed.Shipwreck.X, godSeed.Shipwreck.Z)
		fileSeedOut.Close()
	}
SkipSeedFile:

	ravineAreaX1, ravineAreaZ1, ravineAreaX2, ravineAreaZ2 := godSeed.RavineArea()
	{
		if !UseDocker {
			goto SkipDocker
		}
		tmplComposeMc, err := template.
			New("compose-mc.yml").
			ParseFS(fsTemplate, "template/compose-mc.yml")
		if err != nil {
			log.Fatal(err)
		}
		err = os.MkdirAll("./tmp/mc", os.ModePerm) // todo use exec?
		if err != nil {
			log.Fatal(err)
		}
		fileComposeMc, err := os.Create("./tmp/mc/docker-compose.yml")
		if err != nil {
			log.Fatal(err)
		}
		err = tmplComposeMc.Execute(fileComposeMc, map[string]interface{}{
			"ServiceName": "mc",
			"Seed":        godSeed.Seed,
		})
		if err != nil {
			log.Fatal(err)
		}
		err = fileComposeMc.Close()
		if err != nil {
			log.Fatal(err)
		}

		shutDown := make(chan bool)
		go func(sd chan bool) {
			cmdRmRfWorld := exec.Command("rm", "-rf", "./tmp/mc/data/world")
			if outRmRfWorld, err := cmdRmRfWorld.Output(); err != nil {
				log.Println(string(outRmRfWorld))
				log.Fatal(err)
			}
			cmdComposeMc := exec.Command("docker-compose", "-f", "./tmp/mc/docker-compose.yml", "up")
			if outComposeMc, err := cmdComposeMc.Output(); err != nil {
				log.Println(string(outComposeMc))
				log.Fatal(err)
			}
			sd <- true
		}(shutDown)

		// wait for server to start
		// todo use goroutine
		for {
			log.Println("trying 2 connect 2 srvr")
			cmdEchoMc := exec.Command("docker", "exec", "mc-mc-1", "rcon-cli", "\"msg @p echo\"")
			if _, err := cmdEchoMc.Output(); err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			break
		}

		// forceload chunks
		cmdForceload := exec.Command("docker", "exec", "mc-mc-1", "rcon-cli",
			fmt.Sprintf("forceload add %d %d %d %d", ravineAreaX1, ravineAreaZ1, ravineAreaX2, ravineAreaZ2),
		)
		outForceload, err := cmdForceload.Output()
		log.Println(string(outForceload))
		if err != nil {
			log.Fatal(err)
		}

		// stop server
		cmdStopMc := exec.Command("docker", "exec", "mc-mc-1", "rcon-cli", "stop")
		outStopMc, err := cmdStopMc.Output()
		log.Println(string(outStopMc))
		if err != nil {
			log.Fatal(err)
		}
		<-shutDown
	}
SkipDocker:

	// this part is RLLY bad
	shipwreckAreaX1, shipwreckAreaZ1, shipwreckAreaX2, shipwreckAreaZ2 := godSeed.ShipwreckArea()
	magmaRavineChunks := []Coords{}
	shipwrecksWithIron := []string{}
	for rs := 0; rs < 4; rs++ {
		regionX := (rs % 2) - 1
		regionZ := (rs / 2) - 1

		x1 := ravineAreaX1
		z1 := ravineAreaZ1
		x2 := ravineAreaX2
		z2 := ravineAreaZ2
		if regionX < 0 {
			if x1 >= 0 {
				continue
			}
			if x2 >= 0 {
				x2 = -1
			}
		} else {
			if x2 < 0 {
				continue
			}
			if x1 < 0 {
				x1 = 0
			}
		}
		if regionZ < 0 {
			if z1 >= 0 {
				continue
			}
			if z2 >= 0 {
				z2 = -1
			}
		} else {
			if z2 < 0 {
				continue
			}
			if z1 < 0 {
				z1 = 0
			}
		}

		region, err := region.Open(fmt.Sprintf(
			"./tmp/mc/data/world/region/r.%d.%d.mca",
			regionX,
			regionZ,
		))
		if err != nil {
			log.Fatalf("todo %v", err)
		}

		for xC := x1 / 16; xC < (x2+1)/16; xC++ {
			for zC := z1 / 16; zC < (z2+1)/16; zC++ {
				// todo check sector exists
				// if !region.ExistSector(0, 0) {
				// 	log.Fatal("sector no existo")
				// }
				// log.Println(r.ExistSector(0, 0))
				data, err := region.ReadSector(toSector(xC), toSector(zC))
				if err != nil {
					log.Fatal(err)
				}
				var chunkSave save.Chunk
				err = chunkSave.Load(data)
				if err != nil {
					log.Fatal(err)
				}
				chunkLevel, err := level.ChunkFromSave(&chunkSave)
				if err != nil {
					log.Fatal(err)
				}
				// todo check one layer only
				obby, mb := 0, 0
				for i := 0; i < 16*16*16; i++ {
					x := block.StateList[chunkLevel.Sections[1].GetBlock(i)].ID()
					if x == "minecraft:magma_block" {
						mb++
					}
					if x == "minecraft:obsidian" {
						obby++
					}
				}
				if obby >= 30 && mb >= 10 {
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
				data, err := region.ReadSector(toSector(xC), toSector(zC))
				if err != nil {
					log.Fatal(err)
				}
				var chunkSave save.Chunk
				err = chunkSave.Load(data)
				if err != nil {
					log.Fatal(err)
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
		region.Close()
	}

	log.Println("====================")
	log.Println("> magma ravine")
	if len(magmaRavineChunks) > 0 {
		log.Println(">", magmaRavineChunks)
	}
	log.Println("====================")
	log.Println("> iron shipwreck")
	if len(shipwrecksWithIron) > 0 {
		log.Println(">", shipwrecksWithIron)
	}

	return
}
