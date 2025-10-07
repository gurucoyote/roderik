package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strings"

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
		if err := CurrentElement.Click(proto.InputMouseButtonLeft, 1); err != nil {
			if !navigateViaHrefFallback(err) {
				fmt.Println("Error clicking on the current element:", err)
			}
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
		text := strings.Join(args, " ")
		text = strings.TrimSpace(text)
		if l := len(text); l >= 2 {
			if (text[0] == '"' && text[l-1] == '"') || (text[0] == '\'' && text[l-1] == '\'') {
				text = text[1 : l-1]
			}
		}
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

func navigateViaHrefFallback(clickErr error) bool {
	if Page == nil {
		return false
	}
	hrefAttr, err := CurrentElement.Attribute("href")
	if err != nil || hrefAttr == nil {
		return false
	}
	href := strings.TrimSpace(*hrefAttr)
	if href == "" {
		return false
	}

	base := ""
	if Page != nil {
		if info, infoErr := Page.Info(); infoErr == nil {
			base = info.URL
		} else if Verbose {
			fmt.Fprintf(os.Stderr, "warning: failed to fetch current page info: %v\n", infoErr)
		}
	}
	resolved, err := resolveURL(base, href)
	if err != nil || resolved == "" {
		return false
	}

	page, loadErr := LoadURL(resolved)
	if loadErr != nil {
		fmt.Println("Error clicking on the current element:", clickErr)
		fmt.Println("Fallback navigation failed:", loadErr)
		return true
	}
	Page = page
	if Verbose {
		fmt.Fprintf(os.Stderr, "fallback navigated via href to %s\n", resolved)
	}
	return true
}

func resolveURL(base, href string) (string, error) {
	u, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	if u.IsAbs() {
		return u.String(), nil
	}
	if strings.TrimSpace(base) == "" {
		return "", fmt.Errorf("cannot resolve relative URL %q without a base", href)
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(u).String(), nil
}
