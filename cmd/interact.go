package cmd

import (
	"fmt"
	"github.com/go-rod/rod/lib/proto"
	"github.com/spf13/cobra"
)

var ClickCmd = &cobra.Command{
	Use:   "click",
	Short: "Click on the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		err := CurrentElement.Click(proto.InputMouseButtonLeft, 1)
		if err != nil {
			fmt.Println("Error clicking on the current element:", err)
			return
		}
	},
}

var RClickCmd = &cobra.Command{
	Use:   "rclick",
	Short: "Right click on the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		err := CurrentElement.Click(proto.InputMouseButtonRight, 1)
		if err != nil {
			fmt.Println("Error right clicking on the current element:", err)
			return
		}
	},
}

var TypeCmd = &cobra.Command{
	Use:   "type",
	Short: "Type text into the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		if len(args) < 1 {
			fmt.Println("Error: No text provided for typing")
			return
		}
		text := args[0]
		err := CurrentElement.Input(text)
		if err != nil {
			fmt.Println("Error typing into the current element:", err)
			return
		}
	},
}

func init() {
	RootCmd.AddCommand(ClickCmd)
	RootCmd.AddCommand(RClickCmd)
	RootCmd.AddCommand(TypeCmd)
}
