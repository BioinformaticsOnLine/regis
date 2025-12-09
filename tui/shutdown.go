package tui

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

// ShowGracefulShutdown displays a countdown before exiting
// Returns true if user cancelled the shutdown, false if shutdown completed
func ShowGracefulShutdown() bool {
	fmt.Println()
	fmt.Println("\033[33m(｡•́︿•̀｡) Shutdown requested - terminating all processes...\033[0m")
	fmt.Println("\033[90m(Press any key to cancel, or Ctrl+C to force quit immediately)\033[0m")
	time.Sleep(500 * time.Millisecond)

	countdownEmojis := []string{
		"(｡•́︿•̀｡)", // 5 seconds - sad
		"(´･_･`)",   // 4 seconds - worried
		"(･ัω･ั)",   // 3 seconds - uncertain
		"(っ´ω`)っ",   // 2 seconds - goodbye hug
		"(｡･ω･)ﾉ",   // 1 second - waving
	}

	// Set terminal to raw mode to read individual keypresses
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		// If we can't set raw mode, just do the countdown without cancellation
		for i := 5; i > 0; i-- {
			emojiIndex := 5 - i
			fmt.Printf("\033[33m%s Exiting in %d second", countdownEmojis[emojiIndex], i)
			if i != 1 {
				fmt.Print("s")
			}
			fmt.Print("...\033[0m\n")
			time.Sleep(1 * time.Second)
		}
		fmt.Println("\033[36m(｡･ω･)ﾉﾞ Goodbye! Pipeline terminated. See you next time!\033[0m")
		fmt.Println()
		return false
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Channel for key presses
	keyChan := make(chan bool, 1)
	go func() {
		buf := make([]byte, 1)
		_, err := os.Stdin.Read(buf)
		if err == nil {
			keyChan <- true
		}
	}()

	// Countdown with cancellation check
	cancelled := false
	for i := 5; i > 0; i-- {
		emojiIndex := 5 - i
		fmt.Printf("\033[33m%s Exiting in %d second", countdownEmojis[emojiIndex], i)
		if i != 1 {
			fmt.Print("s")
		}
		fmt.Print("...\033[0m\n")

		// Wait 1 second or until keypress
		select {
		case <-keyChan:
			cancelled = true
			fmt.Println("\033[32m(◕‿◕) Shutdown cancelled!\033[0m")
			fmt.Println()
			term.Restore(int(os.Stdin.Fd()), oldState)
			time.Sleep(500 * time.Millisecond)
			return true
		case <-time.After(1 * time.Second):
			// Continue countdown
		}
	}

	if !cancelled {
		term.Restore(int(os.Stdin.Fd()), oldState)
		fmt.Println("\033[36m(｡･ω･)ﾉﾞ Goodbye! Pipeline terminated. See you next time!\033[0m")
		fmt.Println()
	}

	return false
}
