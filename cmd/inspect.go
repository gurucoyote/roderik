package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var BoxCmd = &cobra.Command{
	Use:   "box",
	Short: "Get the box of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		Box(CurrentElement)
	},
}

func Box(el *rod.Element) error {
	shape, err := el.Shape()
	if err != nil {
		return err
	}
	box := shape.Box()
	fmt.Println("box: ", PrettyFormat(box))
	return nil
}
