package main

import (
	"flag"
	"os"
	"path/filepath"
)

var rootDirFlag = flag.String("root", "", "filesystem root (defaults to CWD or $FS_ROOT)")
var debugFlag = flag.String("debug", "", "write debug logs to this file")
var compatFlag = flag.Bool("compat", false, "return tool results as plain text instead of JSON")

func getRoot() (string, error) {
	var base string
	if *rootDirFlag != "" {
		base = mustAbs(*rootDirFlag)
	} else if env := os.Getenv("FS_ROOT"); env != "" {
		base = mustAbs(env)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		base = mustAbs(cwd)
	}
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	}
	return base, nil
}
