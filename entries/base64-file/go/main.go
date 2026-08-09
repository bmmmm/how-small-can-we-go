package main

import (
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(base64.StdEncoding.EncodeToString(b))
}
