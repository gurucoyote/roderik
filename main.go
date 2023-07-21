package main

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide a URL as a command-line argument.")
		os.Exit(1)
	}
	targetURL := os.Args[1]
	fmt.Println("Target URL:", targetURL)

	// Ensure user data directory exists
	userDataDir := filepath.Join(".", "user_data")
	if _, err := os.Stat(userDataDir); os.IsNotExist(err) {
		os.Mkdir(userDataDir, 0755)
	}

	// Ensure user data directory exists
	userDataDir = filepath.Join(".", "user_data")
	if _, err := os.Stat(userDataDir); os.IsNotExist(err) {
		os.Mkdir(userDataDir, 0755)
	}

	// Get the browser executable path
	path, _ := launcher.LookPath()
	u := launcher.New().Bin(path).
		Set("disable-web-security").
		Set("disable-setuid-sandbox").
		Set("no-sandbox").
		Set("no-first-run", "true").
		Set("disable-gpu").
		Set("user-data-dir", userDataDir).
		Headless(true).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()

	page := browser.MustPage(targetURL)
	fmt.Println("Connected to browser at URL:", page.MustInfo().URL)
	fmt.Println("Opened URL:", page.MustInfo())
	// Get all headings
	headings := page.MustElements("h1, h2, h3, h4, h5, h6")

	// Print the count of headings
	fmt.Println("Count of headings:", len(headings))

	// Print each heading
	for _, heading := range headings {
		// Get the heading level
		level := heading.MustEval("nodeName").String()[1]

		// Get the heading text
		text := heading.MustText()

		fmt.Printf("Heading level %s: %s\n", level, text)
	}
}
