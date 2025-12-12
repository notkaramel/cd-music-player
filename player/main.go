package main

import (
	"log"
	"os"
	"os/exec"
)

func main() {
	// 1. Define the command.
	// We use exec.Command to prepare the execution of the neofetch program.

	cmd := exec.Command("echo", "Hello, World!")

	// 2. Direct the output (Stdout) and error stream (Stderr).
	// By setting Stdout to os.Stdout, the output from neofetch will be
	// printed directly to the standard output of the Go program (your terminal).
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 3. Execute the command.
	// cmd.Run() starts the specified command and waits for it to complete.
	err := cmd.Run()

	if err != nil {
		log.Fatalf("Command failed to run: %v", err)
	}
}
