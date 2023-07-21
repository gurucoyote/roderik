package main

import (
	"bufio"
	"fmt"
	"github.com/eiannone/keyboard"
	"os"
)

func main() {

	fmt.Println("Press any key to see its ASCII code press Q to quit")

	for {
		char, key, err := keyboard.GetSingleKey()
		if err != nil {
			panic(err)
		}

		if char == ':' {
			commandMode()
		} else if key == keyboard.KeyEsc {
			break
		} else if key == keyboard.KeyArrowUp {
			fmt.Println("You pressed: Up Arrow")
		} else if key == keyboard.KeyArrowDown {
			fmt.Println("You pressed: Down Arrow")
		} else if key == keyboard.KeyArrowLeft {
			fmt.Println("You pressed: Left Arrow")
		} else if key == keyboard.KeyArrowRight {
			fmt.Println("You pressed: Right Arrow")
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
