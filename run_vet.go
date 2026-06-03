package main

import (
	"fmt"
	"os/exec"
)

func main() {
	cmd := exec.Command("go", "vet", "./...")
	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))
	if err != nil {
		fmt.Println("Error:", err)
	}
}
