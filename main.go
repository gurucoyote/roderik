package main
import (
    "fmt"
    "os"
    "github.com/go-rod/rod"
    "github.com/go-rod/rod/lib/launcher"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Please provide a URL as a command-line argument.")
        os.Exit(1)
    }
    targetURL := os.Args[1]
    fmt.Println("Target URL:", targetURL)

    path, exists := launcher.LookPath()
    if exists {
        fmt.Println("System Chrome found at:", path)
        url := launcher.New().Bin(path).MustLaunch()
        fmt.Println("Launching system Chrome at URL:", url)
        browser := rod.New().ControlURL(url).MustConnect()
        fmt.Println("Connected to browser at URL:", browser.URL)
        page := browser.MustPage(targetURL)
        fmt.Println("Opened URL:", page.MustInfo().URL)
    } else {
        fmt.Println("System Chrome not found. Launching default browser.")
        url := launcher.New().MustLaunch()
        fmt.Println("Launching default browser at URL:", url)
        browser := rod.New().ControlURL(url).MustConnect()
        fmt.Println("Connected to browser at URL:", browser.URL)
        page := browser.MustPage(targetURL)
        fmt.Println("Opened URL:", page.MustInfo().URL)
    }
}
