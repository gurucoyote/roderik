package main

import (
	"fmt"
	"os"
	"roderik/cmd"
	"strings"

	"github.com/chzyer/readline"
	"golang.org/x/term"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if cmd.Interactive && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		// enter repl loop
		rl, _ := readline.New("> ")
		defer rl.Close()
		for {
			input, _ := rl.Readline()
			args := strings.Fields(input)
			cmd.RootCmd.SetArgs(args)
			cmd.RootCmd.Execute()
		}
	}
}
