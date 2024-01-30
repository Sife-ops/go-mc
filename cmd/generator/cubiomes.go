package main

import (
	"context"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func Cubiomes() {
	for {
		<-time.After(100 * time.Millisecond)

		if len(CubiomesDone) >= *FlagJobs {
			return
		}
		todo := *FlagJobs - len(CubiomesDone)
		if todo >= *FlagThreads {
			if len(CubiomesInProg) >= *FlagThreads {
				continue
			}
		} else {
			if len(CubiomesInProg) >= todo {
				continue
			}
		}

		CubiomesInProg <- struct{}{}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			log.Println("info starting new cubiomes thread")
			outCubiomes, err := exec.CommandContext(ctx, "./cubiomes").Output()
			outCubiomesArr := strings.Split(string(outCubiomes), ":")
			if err != nil {
				log.Printf("warning cubiomes job exited %v", err)
				goto CubiomesExit
			}
			log.Printf("info cubiomes output: %s", string(outCubiomes))

			{
				gs := GodSeed{
					Seed:             ToStringRef(outCubiomesArr[0]),
					SpawnX:           MustIntRef(strconv.Atoi(strings.Split(outCubiomesArr[1], ",")[0])),
					SpawnZ:           MustIntRef(strconv.Atoi(strings.Split(outCubiomesArr[1], ",")[1])),
					ShipwreckX:       MustIntRef(strconv.Atoi(strings.Split(outCubiomesArr[2], ",")[0])),
					ShipwreckZ:       MustIntRef(strconv.Atoi(strings.Split(outCubiomesArr[2], ",")[1])),
					BastionX:         MustIntRef(strconv.Atoi(strings.Split(outCubiomesArr[3], ",")[0])),
					BastionZ:         MustIntRef(strconv.Atoi(strings.Split(outCubiomesArr[3], ",")[1])),
					FortressX:        MustIntRef(strconv.Atoi(strings.Split(outCubiomesArr[4], ",")[0])),
					FortressZ:        MustIntRef(strconv.Atoi(strings.Split(outCubiomesArr[4], ",")[1])),
					FinishedCubiomes: ToIntRef(1),
				}

				if _, err := Db.NamedExec(
					`INSERT INTO seed (
						seed, spawn_x, spawn_z, bastion_x, bastion_z, shipwreck_x, shipwreck_z, fortress_x, fortress_z, finished_cubiomes) 
					VALUES 
						(:seed, :spawn_x, :spawn_z, :bastion_x, :bastion_z, :shipwreck_x, :shipwreck_z, :fortress_x, :fortress_z, :finished_cubiomes)`,
					&gs,
				); err != nil {
					log.Fatalf("error %v", err)
				}

				CubiomesOut <- gs
			}

			CubiomesDone <- struct{}{}
			log.Printf("info finished cubiomes job %d", len(CubiomesDone))

		CubiomesExit:
			log.Println("info freeing cubiomes thread")
			<-CubiomesInProg
		}()
	}

}
