package main

import (
	"fmt"
	"github.com/eiannone/keyboard"
)

func main() {
	err := keyboard.Open()
	if err != nil {
		panic(err)
	}
	defer keyboard.Close()

	fmt.Println("Press any key to see its ASCII code press Q to quit")

	commandMode := false
	command := ""

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			panic(err)
		}

		if commandMode {
			if key == keyboard.KeyEnter {
				fmt.Printf("Command entered: %s\r\n", command)
				command = ""
				commandMode = false
			} else {
				command += string(char)
			}
		} else {
			fmt.Printf("You pressed: rune %q, key %X\r\n", char, key)

			if char == ':' {
				commandMode = true
			} else if key == keyboard.KeyEsc {
				break
			}
		}
	}
}
