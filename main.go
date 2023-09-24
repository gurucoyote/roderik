package main

import (
	"fmt"
	"os"
	"roderik/cmd"
	"strings"

	"github.com/chzyer/readline"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if cmd.Interactive {
		// enter repl loop
		rl, _ := readline.New("> ")
		defer rl.Close()
		for {
			input, _ := rl.Readline()
			if input == "exit" {
				fmt.Println("Goodbye")
				break
			}

			args := strings.Fields(input)
			cmd.RootCmd.SetArgs(args)
			cmd.RootCmd.Execute()
		}
	}
}
