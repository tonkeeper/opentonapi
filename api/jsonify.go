package main

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		panic("not enough args")
	}

	inputFile, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	var v any
	err = yaml.NewDecoder(inputFile).Decode(&v)
	if err != nil {
		panic(err)
	}
	outFile, err := os.Create(os.Args[2])
	if err != nil {
		panic(err)
	}
	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", " ")
	err = encoder.Encode(v)
	if err != nil {
		panic(err)
	}
	outFile.Close()
}
