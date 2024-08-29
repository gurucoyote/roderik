package cmd

import (
	"fmt"

	"github.com/go-rod/rod"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(SearchCmd)
	RootCmd.AddCommand(FirstCmd)
	RootCmd.AddCommand(LastCmd)
	RootCmd.AddCommand(NsCmd)
	RootCmd.AddCommand(PsCmd)
	RootCmd.AddCommand(WalkCmd)
	RootCmd.AddCommand(ParentCmd)
	RootCmd.AddCommand(ChildCmd)
	RootCmd.AddCommand(HeadCmd)
	RootCmd.AddCommand(BodyCmd)
	RootCmd.AddCommand(ElemCmd)
}

var ElemCmd = &cobra.Command{
	Use:   "elem [selector]",
	Short: "Navigate to the first element that matches the CSS selector",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		selector := args[0]
		element, err := CurrentElement.Element(selector)
		if err != nil {
			fmt.Println("Error navigating to the element:", err)
			return
		}
		CurrentElement = element
		fmt.Println("Navigated to the element.")
		ReportElement(CurrentElement)
	},
}

var BodyCmd = &cobra.Command{
	Use:   "body",
	Short: "Navigate to the document's body",
	Run: func(cmd *cobra.Command, args []string) {
		bodyElement, err := Page.Element("body")
		if err != nil {
			fmt.Println("Error navigating to the document's body:", err)
			return
		}
		CurrentElement = bodyElement
		fmt.Println("Navigated to the document's body.")
		ReportElement(CurrentElement)
	},
}

var HeadCmd = &cobra.Command{
	Use:   "head [level]",
	Short: "Navigate to the first heading of the specified level, or any level if none is specified",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		selector := "h1, h2, h3, h4, h5, h6"
		if len(args) > 0 {
			selector = fmt.Sprintf("h%s", args[0])
		}
		headings, err := Page.Elements(selector)
		if err != nil {
			fmt.Println("couldn't find any heading of type ", selector, err)
			return
		}
		if len(headings) > 0 {
			CurrentElement = headings[0]
		}
		ReportElement(CurrentElement)
	},
}

var elementList []*rod.Element
var currentIndex int

var SearchCmd = &cobra.Command{
	Use:   "search [selector]",
	Short: "Search for elements matching the CSS selector and build an internal list",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		selector := args[0]
		elements, err := Page.Elements(selector)
		if err != nil {
			fmt.Println("Error searching for elements:", err)
			return
		}
		elementList = elements
		if len(elementList) > 0 {
			currentIndex = 0
			CurrentElement = elementList[currentIndex]
			fmt.Println("Found elements. Navigated to the first element.")
			ReportElement(CurrentElement)
		} else {
			fmt.Println("No elements found.")
		}
	},
}

var FirstCmd = &cobra.Command{
	Use:   "first",
	Short: "Navigate to the first element in the list",
	Run: func(cmd *cobra.Command, args []string) {
		if len(elementList) == 0 {
			fmt.Println("Element list is empty. Please perform a search first.")
			return
		}
		currentIndex = 0
		CurrentElement = elementList[currentIndex]
		fmt.Println("Navigated to the first element.")
		ReportElement(CurrentElement)
	},
}

var LastCmd = &cobra.Command{
	Use:   "last",
	Short: "Navigate to the last element in the list",
	Run: func(cmd *cobra.Command, args []string) {
		if len(elementList) == 0 {
			fmt.Println("Element list is empty. Please perform a search first.")
			return
		}
		currentIndex = len(elementList) - 1
		CurrentElement = elementList[currentIndex]
		fmt.Println("Navigated to the last element.")
		ReportElement(CurrentElement)
	},
}

var NextCmd = &cobra.Command{
	Use:   "next",
	Short: "Navigate to the next element in the list",
	Run: func(cmd *cobra.Command, args []string) {
		if len(elementList) == 0 {
			fmt.Println("Element list is empty. Please perform a search first.")
			return
		}
		if currentIndex < len(elementList)-1 {
			currentIndex++
			CurrentElement = elementList[currentIndex]
			fmt.Println("Navigated to the next element.")
			ReportElement(CurrentElement)
		} else {
			fmt.Println("Already at the last element.")
		}
	},
}

var PrevCmd = &cobra.Command{
	Use:   "prev",
	Short: "Navigate to the previous element in the list",
	Run: func(cmd *cobra.Command, args []string) {
		if len(elementList) == 0 {
			fmt.Println("Element list is empty. Please perform a search first.")
			return
		}
		if currentIndex > 0 {
			currentIndex--
			CurrentElement = elementList[currentIndex]
			fmt.Println("Navigated to the previous element.")
			ReportElement(CurrentElement)
		} else {
			fmt.Println("Already at the first element.")
		}
	},
}

func init() {
	RootCmd.AddCommand(SearchCmd)
	RootCmd.AddCommand(FirstCmd)
	RootCmd.AddCommand(LastCmd)
	RootCmd.AddCommand(NsCmd)
	RootCmd.AddCommand(PsCmd)
	RootCmd.AddCommand(WalkCmd)
	RootCmd.AddCommand(ParentCmd)
	RootCmd.AddCommand(ChildCmd)
	RootCmd.AddCommand(HeadCmd)
	RootCmd.AddCommand(BodyCmd)
	RootCmd.AddCommand(ElemCmd)
	RootCmd.AddCommand(NextCmd)
	RootCmd.AddCommand(PrevCmd)
}

var WalkCmd = &cobra.Command{
	Use:   "walk",
	Short: "Walk to the next element for a number of steps",
	Run: func(cmd *cobra.Command, args []string) {
		if CurrentElement == nil {
			return
		}
		steps, _ := cmd.Flags().GetInt("steps")
		if steps < 0 {
			steps = -steps
			for i := 0; i < steps; i++ {
				PsCmd.Run(cmd, []string{})
			}
		} else {
			for i := 0; i < steps; i++ {
				NsCmd.Run(cmd, []string{})
			}
		}
	},
}

func init() {
	WalkCmd.Flags().Int("steps", 4, "Number of steps to walk")
}

var PsCmd = &cobra.Command{
	Use:   "ps",
	Short: "Navigate to the previous element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		prevElement, err := CurrentElement.Previous()
		if err != nil {
			fmt.Println("Error navigating to the previous element:", err)
			return
		}
		CurrentElement = prevElement
		ReportElement(prevElement)
	},
}

var ChildCmd = &cobra.Command{
	Use:   "child",
	Short: "Navigate to the first child of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		childElement := CurrentElement.MustElement(":first-child")
		CurrentElement = childElement
		fmt.Println("Navigated to the child element.")
		ReportElement(CurrentElement)
	},
}

var ParentCmd = &cobra.Command{
	Use:   "parent",
	Short: "Navigate to the parent of the current element",
	Run: func(cmd *cobra.Command, args []string) {
		if !hasCurrentElement() {
			return
		}
		parentElement, err := CurrentElement.Parent()
		if err != nil {
			fmt.Println("Error navigating to the parent element:", err)
			return
		}
		CurrentElement = parentElement
		fmt.Println("Navigated to the parent element.")
		ReportElement(CurrentElement)
	},
}
