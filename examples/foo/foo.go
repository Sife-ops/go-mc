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
	goto SkipCubiomes
SkipCubiomes:

	{
		fileSeedOut, err := os.Create("./seed.txt")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(fileSeedOut, "seed: %s\n", godSeed.Seed)
		fmt.Fprintf(fileSeedOut, "spawn: %d, %d\n", godSeed.Spawn.X, godSeed.Spawn.Z)
		fmt.Fprintf(fileSeedOut, "shipwreck: %d, %d\n", godSeed.Shipwreck.X, godSeed.Shipwreck.Z)
		fileSeedOut.Close()
	}
	goto SkipSeedFile
SkipSeedFile:

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
	cmdForceload := exec.Command(
		"docker",
		"exec",
		"mc-mc-1",
		"rcon-cli",
		fmt.Sprintf(
			"forceload add %d %d %d %d",
			godSeed.Shipwreck.X-112,
			godSeed.Shipwreck.Z-112,
			godSeed.Shipwreck.X+127,
			godSeed.Shipwreck.Z+127,
		),
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

	// this part is RLLY bad
	for rs := 0; rs < 4; rs++ {
		regionX := (rs % 2) - 1
		regionZ := (rs / 2) - 1

		x1 := godSeed.Shipwreck.X - 112
		z1 := godSeed.Shipwreck.Z - 112
		x2 := godSeed.Shipwreck.X + 127
		z2 := godSeed.Shipwreck.Z + 127

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
					log.Fatalf("todo %v", err)
				}
				if err != nil {
					log.Fatal(err)
				}
				var chunkSave save.Chunk
				err = chunkSave.Load(data)
				if err != nil {
					log.Fatalf("todo %v", err)
				}
				chunkLevel, err := level.ChunkFromSave(&chunkSave)
				if err != nil {
					log.Fatalf("todo %v", err)
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
					log.Printf("magma ravine %d,%d", xC, zC)
				}
			}
		}

		region.Close()
	}

	// todo read structure data 4 shipwreck variant
	// todo detect hard block above chest
	// var chests []int
	// for o := 0; o < 9; o++ {
	// 	sectorX := toSector(godSeed.Shipwreck.X) - 1 + (o % 3)
	// 	sectorZ := toSector(godSeed.Shipwreck.Z) - 1 + (o / 3)
	// 	if sectorX < 0 || sectorZ < 0 {
	// 		log.Println("warning: shipwreck may be in another region!")
	// 		continue
	// 	}
	// 	data, err := region.ReadSector(sectorX, sectorZ)
	// 	if err != nil {
	// 		log.Fatalf("todo %v", err)
	// 	}
	// 	var chunkSave save.Chunk
	// 	err = chunkSave.Load(data)
	// 	if err != nil {
	// 		log.Fatalf("todo %v", err)
	// 	}
	// 	chunkLevel, err := level.ChunkFromSave(&chunkSave)
	// 	if err != nil {
	// 		log.Fatalf("todo %v", err)
	// 	}
	// 	// section 2 to 4 (3 to 5)
	// 	for section := 3; section < 6; section++ {
	// 		for i := 0; i < 16*16*16; i++ {
	// 			x := block.StateList[chunkLevel.Sections[section].GetBlock(i)].ID()
	// 			if x == "minecraft:chest" {
	// 				log.Printf("chest detected! %d", i)
	// 				chests = append(chests, i)
	// 			}
	// 		}
	// 	}
	// }

	return
}
