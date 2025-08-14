package main

import (
	"fmt"
	"log"
	"os"
	"sync"
)

var (
	debugEnabled bool
	debugMu      sync.Mutex
	debugLog     *log.Logger
)

func initDebug() {
	if *debugFlag == "" {
		return
	}
	f, err := os.Create(*debugFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		return
	}
	debugEnabled = true
	debugLog = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
}

func dprintf(format string, args ...any) {
	if !debugEnabled || debugLog == nil {
		return
	}
	debugMu.Lock()
	defer debugMu.Unlock()
	debugLog.Printf(format, args...)
}
