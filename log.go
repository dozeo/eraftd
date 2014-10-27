package eraftd

import (
	golog "log"
	"os"
)

type loginterface interface {
	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatalln(v ...interface{})
	Panic(v ...interface{})
	Panicf(format string, v ...interface{})
	Panicln(v ...interface{})
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

var Logger loginterface

func init() {
	Logger = golog.New(os.Stdout, "", 0)
}
