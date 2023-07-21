package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/utils"
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
    path, exists := launcher.LookPath()
    if !exists {
        fmt.Println("Browser executable not found.")
        os.Exit(1)
    }

    // use the FormatArgs to construct args, this line is optional, you can construct the args manually
    l := launcher.New().Bin(path).UserDataDir(userDataDir)
    args := l.FormatArgs()

    var cmd *exec.Cmd
    cmd = exec.Command(path, args...)

    parser := launcher.NewURLParser()
    cmd.Stderr = parser
    utils.E(cmd.Start())
    u := launcher.MustResolveURL(<-parser.URL)

	browser := rod.New().ControlURL(u).MustConnect()

	page := browser.MustPage(targetURL)
	fmt.Println("Connected to browser at URL:", page.MustInfo().URL)
	fmt.Println("Opened URL:", page.MustInfo().URL)
}
