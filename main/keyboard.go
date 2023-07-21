package main

import (
	"bufio"
	"fmt"
	"github.com/eiannone/keyboard"
	"os"
)

func main() {
	err := keyboard.Open()
	if err != nil {
		panic(err)
	}
	defer keyboard.Close()

	fmt.Println("Press any key to see its ASCII code press Q to quit")

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			panic(err)
		}

		if char == ':' {
			commandMode()
		} else if key == keyboard.KeyEsc {
			break
		} else {
			fmt.Printf("You pressed: rune %q, key %X\r\n", char, key)
		}
	}
}

func commandMode() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter command: ")
	command, _ := reader.ReadString('\n')
	fmt.Printf("Command entered: %s\r\n", command)
}
