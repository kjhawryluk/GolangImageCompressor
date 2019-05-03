package main

import (
	cp "compressionprocess"
	"fmt"
	"os"
	r "regexp"
	"runtime"
	"strconv"
)

func main() {
	args := os.Args
	if len(args) < 2 {
		fmt.Println("No CSV Provided")
		return
	}
	csvPath := args[1]
	re := r.MustCompile("p=(\\d+)")
	if len(args) < 3 {
		fmt.Println("Running Sequential Application...")
		cp.LaunchSeqApplication(csvPath)
		return
	}
	var numCpusInt int
	var e error
	// Parse Flags
	numCpus := re.FindStringSubmatch(args[2])
	if numCpus != nil {
		numCpusInt, e = strconv.Atoi(numCpus[1])
		if e != nil {
			fmt.Println("Invalid Arguments. For parralel processing please include p or p=[number of threads]")
			return
		}
	}

	// Run with default number of threads or user provided
	if args[2] == "-p" {
		fmt.Println("Running Parralel Application With", runtime.NumCPU(), " threads...")
		cp.LaunchConcurrentApplication(runtime.NumCPU(), csvPath)
	} else {
		fmt.Println("Running Parralel Application With", numCpusInt, " threads...")
		if numCpusInt > 1 {
			cp.LaunchConcurrentApplication(numCpusInt, csvPath)
		} else {
			cp.LaunchSeqApplication(csvPath)
		}
	}

}
