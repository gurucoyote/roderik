package main

import (
	"fmt"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

var ShowNetActivity bool

type EventLog struct {
	mu    sync.Mutex
	logs  []string
}

func (l *EventLog) Add(log string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, log)
}

func (l *EventLog) Display() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, log := range l.logs {
		fmt.Println(log)
	}
}

func main() {
	browser := rod.New().MustConnect()
	defer browser.MustClose()

	page := browser.MustPage()

	eventLog := &EventLog{}

	page.EnableDomain(proto.NetworkEnable{})
	go page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
		if ShowNetActivity {
			fmt.Printf("Request sent: %s\n", e.Request.URL)
		} else {
			eventLog.Add(fmt.Sprintf("Request sent: %s", e.Request.URL))
		}
	})()
	go page.EachEvent(func(e *proto.NetworkResponseReceived) {
		if ShowNetActivity {
			fmt.Printf("Response received: %s Status: %d\n", e.Response.URL, e.Response.Status)
		} else {
			eventLog.Add(fmt.Sprintf("Response received: %s Status: %d", e.Response.URL, e.Response.Status))
		}
	})()

	page.MustNavigate("https://example.com")

	// Display the logs
	if !ShowNetActivity {
		eventLog.Display()
	}
}
