package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"golang.org/x/term"
)

const (
	messageSize = 8192
)

func (cfg *apiConfig) ReadLoop(d io.Reader, cancel context.CancelFunc) {
	time.Sleep(200 * time.Millisecond)

    // Put local terminal into raw mode
    oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
    if err != nil {
        log.Fatal(err)
    }
    defer term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Print("\r\n")

	time.Sleep(200 * time.Millisecond)
	buffer := make([]byte, messageSize)
	for {
		n, err := d.Read(buffer)
		if err != nil {
			// fmt.Printf("data channel closed: %s\n", err)
			return
		}
		//fmt.Printf("Received: %s", buffer[:n])
		os.Stdout.Write(buffer[:n])
	}
}

func (cfg *apiConfig) WriteLoop(d io.Writer, cancel context.CancelFunc) {
	time.Sleep(200 * time.Millisecond)
	buffer := make([]byte, messageSize)
	for {
		n, err := os.Stdin.Read(buffer)
		if err != nil {
			return
		}
		if n > 0 {
			_, err = d.Write(buffer[:n])
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
