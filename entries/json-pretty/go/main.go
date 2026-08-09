package main

import (
	"bytes"
	"encoding/json"
	"os"
)

func main() {
	b, _ := os.ReadFile(os.Args[1])
	if !json.Valid(b) {
		os.Exit(1)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	d.Decode(&v)
	e := json.NewEncoder(os.Stdout)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	e.Encode(v)
}
