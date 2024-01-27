package main

import (
	"math"
)

type Coords struct {
	X int
	Z int
}

type GodSeed struct {
	Seed      string // todo mb use int?
	Spawn     Coords
	Shipwreck Coords
	Bastion   Coords
	Fortress  Coords
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

func (g *GodSeed) NetherChunksToBastion() (netherChunks2Load []Coords) {
	bz, bx := g.Bastion.Z+8, g.Bastion.X+8
	// log.Printf("info bastion chunk center coords %d,%d", bx, bz)
	s := float64(bz) / float64(bx)
	// log.Printf("info bastion slope %f", s)
	bxa := math.Abs(float64(bx))

	for i := 1; i < int(bxa); i++ {
		x := i
		if bx < 0 {
			x = i * -1
		}

		a, b := int(math.Floor(float64(x)/16)), int(math.Floor(float64(x)*s/16))
		hasChunk := false
		for _, v := range netherChunks2Load {
			if v.X == a && v.Z == b {
				hasChunk = true
			}
		}
		if hasChunk == false {
			netherChunks2Load = append(netherChunks2Load, Coords{a, b})
		}
	}
	return
}

func MustInt(i int, err error) int {
	if err != nil {
		panic(err)
	}
	return i
}

func MustString(i string, err error) string {
	if err != nil {
		panic(err)
	}
	return i
}

func ToSector(i int) int {
	if i < 0 {
		return 32 + i
	}
	return i
}
