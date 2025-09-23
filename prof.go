package jote

import (
	"fmt"
	"os"
	"runtime/pprof"
	"time"
)

// Starts profiling using the pprof package if the PPROF environment variable is set.
// The target file is the value of the pprof envvar.
func ProfilingUntilTimeIfSet(duration int) {
	pprofLocation := os.Getenv("PPROF")
	if pprofLocation == "" {
		return
	}
	fmt.Println("DEBUG/jote starting profiling")
	f, err := os.Create(pprofLocation)
	if err != nil {
		panic(err)
	}
	// runtime.SetCPUProfileRate(1000000)
	if err := pprof.StartCPUProfile(f); err != nil {
		panic(err)
	}
	go func() {
		time.Sleep(time.Duration(duration) * time.Second)
		fmt.Println("DEBUG/jote stopping profiling")
		pprof.StopCPUProfile()
	}()
}
