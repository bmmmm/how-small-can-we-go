package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		os.Exit(1)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		os.Exit(1)
	}
	fmt.Printf("%x\n", h.Sum(nil))
}
