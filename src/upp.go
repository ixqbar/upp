package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"upp"
)

var version = flag.Bool("version", false, "print current version")
var optionConfigFile = flag.String("config", "./config.xml", "configure xml file")
var verbose = flag.Bool("verbose", true, "show running logs")
var help = flag.Bool("help", false, "show help")

func usage() {
	fmt.Printf("Usage: %s [options]Options:", os.Args[0])
	flag.PrintDefaults()
	os.Exit(0)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if len(os.Args) < 2 || *help {
		usage()
	}

	if *version {
		fmt.Printf("%s\n", upp.VERSION)
		os.Exit(0)
	}

	if *verbose == false {
		upp.Logger.SetOutput(ioutil.Discard)
	}

	_, err := upp.ParseXmlConfig(*optionConfigFile)
	if err != nil {
		upp.Logger.Print(err)
		os.Exit(1)
	}

	upp.Run()
}
