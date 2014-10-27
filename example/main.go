package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/dozeo/eraftd"
)

var pverbose bool
var phost string
var pport int
var pjoin string
var ppath string

func init() {
	flag.BoolVar(&pverbose, "v", false, "verbose logging")
	flag.StringVar(&phost, "h", "localhost", "hostname")
	flag.IntVar(&pport, "p", 4001, "port")
	flag.StringVar(&pjoin, "join", "", "host:port of leader to join")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [arguments] <data-path> \n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Println("Path missing")
		os.Exit(1)
	}
	ppath = flag.Arg(0)
	rand.Seed(time.Now().UnixNano())
}

func main() {
	db := dbNew()
	cdb := eraftd.StartCluster(pport, phost, pjoin, db, ppath)
	// cdb us this to read and write to the cluster
	fmt.Println("Cluster node ready")
	cdb.Read([]string{"hi"})
	x, err := cdb.Write([]string{"a", "b"})
	fmt.Println("WR", x, err)
	x, err = cdb.Write([]string{"a", "b"})
	fmt.Println("WR", x, err)
	y, err := cdb.Read([]string{"a"})
	fmt.Println("READ", "a => ", y, err)
	// ----------------------------------------
	// capture ctrl+c
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	for sig := range c {
		log.Printf("\nShutting down System %v .", sig)
		time.Sleep(1 * time.Second)
		log.Printf(". \ndone")
		break
	}
	log.Printf("\nexit")
}
