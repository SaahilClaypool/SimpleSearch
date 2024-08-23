package main

import (
	"fmt"
	"os"

	"github.com/saahilclaypool/simplesearch/agent"
)

func main() {
	input := os.Args[1]
	res, err := agent.Research(input, 3, 5)
	if err != nil {
		panic(err)
	}
	fmt.Printf("================\nResearch:\n%s", res.Answer)
}
