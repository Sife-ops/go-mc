package main

import (
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var Db *sqlx.DB

type Seed struct {
	Id             int    `db:"id"`
	Seed           string `db:"seed"`
	RavineChunks   int    `db:"ravine_chunks"`
	IronShipwrecks int    `db:"iron_shipwrecks"`
	Played         int    `db:"played"`
	AvgBastionAir  *int   `db:"abg_bastion_air"`
	Rating         *int   `db:"rating"`
	Notes          *int   `db:"notes"`
	Timestamp      string `db:"timestamp"`
}

func init() {
	if _, e := os.Stat("./db.sqlite"); e != nil {
		log.Fatalf("%v", e)
	}
	db, e := sqlx.Open("sqlite3", "./db.sqlite")
	if e != nil {
		log.Fatalf("%v", e)
	}
	Db = db
}
