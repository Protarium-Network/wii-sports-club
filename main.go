package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"sync"

	"github.com/Protarium-Network/wii-sports-club/nex"
)

var wg sync.WaitGroup

// logPanic recovers from a panic in a critical goroutine, logs it, and
// terminates the whole process. Continuing with a half-dead server (e.g.
// Secure server down but Auth server still up) would silently break the
// game connection without any visible indication.
func logPanic(serverName string) {
	if r := recover(); r != nil {
		stack := string(debug.Stack())
		msg := fmt.Sprintf("[PANIC CRASH] %s crashed: %v\nStack trace:\n%s\n", serverName, r, stack)
		fmt.Print(msg)
		_ = os.WriteFile("crash_panic.txt", []byte(msg), 0644)
		os.Exit(1)
	}
}

func main() {
	defer logPanic("Main Thread")
	wg.Add(2)

	go func() {
		defer logPanic("Auth Server")
		nex.StartAuthenticationServer()
	}()

	go func() {
		defer logPanic("Secure Server")
		nex.StartSecureServer()
	}()

	wg.Wait()
}
