package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/dozeo/eraftd"
	log "github.com/kdar/factorlog"
)

var pverbose bool
var phost string
var pport int
var pjoin string
var ppath string

func init() {
	pid := strconv.Itoa(os.Getpid())
	frmt := []string{`%{Color "red" "ERROR"}%{Color "yellow" "WARN"}%{Color "green" "INFO"}%{Color "cyan" "DEBUG"}%{Color "blue" "TRACE"}[%{Date} %{Time}] `, pid, ` [%{SEVERITY}:%{File}:%{Line}] %{Message}%{Color "reset"}`}
	frmt2 := strings.Join(frmt, "")
	log.SetFormatter(log.NewStdFormatter(frmt2))
	eraftd.Logger = log.New(os.Stdout, log.NewStdFormatter(frmt2))
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
	key := strconv.Itoa(pport)
	db := dbNew()
	cdb := eraftd.StartCluster(pport, phost, pjoin, db, ppath)
	// cdb us this to read and write to the cluster
	log.Println("Cluster node ready")
	x, err := cdb.Write([]string{key, "b"})
	log.Warnln("main.WRITE ", x, " ", err)
	time.Sleep(100 * time.Millisecond)
	y, err := cdb.Read([]string{key})
	log.Infoln("main.READ ", key, " => ", y, " ", err)
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
