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

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			panic(err)
		}

		fmt.Printf("You pressed: rune %q, key %X\r\n", char, key)

		if key == keyboard.KeyEsc {
			break
		}
	}
}
