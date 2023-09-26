package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/go-rod/rod"
)

var A11yCmd = &cobra.Command{
	Use:   "a11y",
	Short: "Access the accessibility tree of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		ReportElement(CurrentElement)
	},
}

func init() {
	RootCmd.AddCommand(A11yCmd)
}
