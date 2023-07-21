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

		switch {
		case char == ':':
			commandMode()
		case key == keyboard.KeyEsc:
			break
		case key == keyboard.KeyArrowUp:
			fmt.Println("You pressed: Up Arrow")
		case key == keyboard.KeyArrowDown:
			fmt.Println("You pressed: Down Arrow")
		case key == keyboard.KeyArrowLeft:
			fmt.Println("You pressed: Left Arrow")
		case key == keyboard.KeyArrowRight:
			fmt.Println("You pressed: Right Arrow")
		default:
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
