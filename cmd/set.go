package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"strconv"
)

var SetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set or toggle the value of a command-line flag",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		flagName := args[0]
		flag := RootCmd.PersistentFlags().Lookup(flagName)
		if flag == nil {
			fmt.Println("Error: unknown flag:", flagName)
			return
		}
		var newValue string
		if len(args) > 1 {
			newValue = args[1]
		} else if flag.Value.Type() == "bool" {
			oldValue, _ := strconv.ParseBool(flag.Value.String())
			newValue = strconv.FormatBool(!oldValue)
		} else {
			fmt.Println("Error: no value provided for non-boolean flag:", flagName)
			return
		}
		if err := flag.Value.Set(newValue); err != nil {
			fmt.Println("Error setting flag:", err)
			return
		}
		fmt.Println("Set", flagName, "to", flag.Value)
	},
}

func init() {
	RootCmd.AddCommand(SetCmd)
}
