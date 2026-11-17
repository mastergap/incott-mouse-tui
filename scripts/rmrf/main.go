// Command rmrf removes the given paths; used by Taskfile for cross-platform clean.
package main

import (
	"fmt"
	"os"
)

func main() {
	for _, p := range os.Args[1:] {
		if err := os.RemoveAll(p); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
