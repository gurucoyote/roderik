import (
    "github.com/go-rod/rod"
    "github.com/go-rod/rod/lib/launcher"
)

func main() {
    url := launcher.New().MustLaunch()
    browser := rod.New().ControlURL(url).MustConnect()
}
