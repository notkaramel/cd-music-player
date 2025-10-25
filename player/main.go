package main

import (
	"fmt"
	"log"
	"os/exec"
)

func main() {
	cmd := exec.Command("cdda2wav", "-v", "255", "-B")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	}
	fmt.Println(string(output))
}
