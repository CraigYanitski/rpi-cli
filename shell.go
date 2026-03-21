package main

import (
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

func (cfg *apiConfig) ReadLoop(d io.Reader) {
	time.Sleep(100 * time.Millisecond)

    // Put local terminal into raw mode
    oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
    if err != nil {
        log.Println(err)
		close(cfg.closeChan)
		return
    }
    defer term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Print("\r\n")

	buffer := make([]byte, messageSize)
	for {
		n, err := d.Read(buffer)
		if err != nil {
			fmt.Println("")
			log.Printf("(read) data channel closed: %s\r\n", err)
			close(cfg.closeChan)
			return
		}
		os.Stdout.Write(buffer[:n])
	}
}

func (cfg *apiConfig) WriteLoop(d io.Writer) {
	buffer := make([]byte, messageSize)
	for {
		n, err := os.Stdin.Read(buffer)
		if err != nil {
			fmt.Println("")
			log.Printf("(write) data channel closed: %s\r\n", err)
			close(cfg.closeChan)
			return
		}
		if n > 0 {
			_, err = d.Write(buffer[:n])
			if err != nil {
				cfg.closeChan<- true
				return
			}
		}
	}
}
