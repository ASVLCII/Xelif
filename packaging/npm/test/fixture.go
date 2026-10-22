//go:build ignore

// A disposable executable for testing the launcher, not part of Xelif.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "wait" {
		fmt.Println("ready")
		time.Sleep(time.Hour)
		return
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	result := struct {
		Args  []string `json:"args"`
		Cwd   string   `json:"cwd"`
		Input string   `json:"input"`
		Env   string   `json:"env"`
	}{os.Args[1:], cwd, string(input), os.Getenv("XELIF_LAUNCHER_TEST")}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		panic(err)
	}
	fmt.Fprint(os.Stderr, "fixture stderr")
	os.Exit(37)
}
