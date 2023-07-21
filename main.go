package main

import (
	"fmt"
	"os"
	"roderik/cmd"
)

func init() {
	cmd.RootCmd.AddCommand(cmd.NextCmd)
	cmd.RootCmd.AddCommand(cmd.PrevCmd)
}

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

