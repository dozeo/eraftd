package eraftd

import (
	golog "log"
	"os"
)

type logInterface interface {
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

var Logger logInterface

func init() {
	Logger = golog.New(os.Stdout, "", 0)
}
