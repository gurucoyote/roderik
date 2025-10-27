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
	if cmd.Interactive && cmd.StdinIsTerminal() && cmd.StdoutIsTerminal() {
		// enter repl loop
		rl, _ := readline.New("> ")
		defer rl.Close()
		for {
			input, _ := rl.Readline()
			cmd.LogUserInput(input)
			args := strings.Fields(input)
			cmd.RootCmd.SetArgs(args)
			cmd.RootCmd.Execute()
		}
	}
}
