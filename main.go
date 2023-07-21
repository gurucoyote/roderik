package main

import (
	"encoding/json"
	"fmt"
	"os"
	"roderik/cmd"
)

func init() {
	rootCmd.AddCommand(cmd.NextCmd)
	rootCmd.AddCommand(cmd.PrevCmd)
}
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
// PrettyFormat function

// prettyPrintJson function
func prettyPrintJson(s string) string {
	var i interface{}
	json.Unmarshal([]byte(s), &i)
	b, _ := json.MarshalIndent(i, "", "  ")
	return string(b)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
