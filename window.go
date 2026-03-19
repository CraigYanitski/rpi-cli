package main

import (
	"encoding/json"
	"fmt"
	"log"
	//"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/pion/webrtc/v4"
	"golang.org/x/term"
)

type WindowSize struct {
	Cols  uint16  `json:"cols"`
	Rows  uint16  `json:"rows"`
	ColsChanged  bool  `json:"colsChanged"`
	RowsChanged  bool  `json:"rowsChanged"`
}

func getWindowSize() (*WindowSize, error) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return nil, err
	}
	ws := &WindowSize{
		Cols: uint16(width),
		Rows: uint16(height),
		ColsChanged: true,
		RowsChanged: true,
	}
	return ws, nil
}

func (cfg *apiConfig) watchResize(d *webrtc.DataChannel) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	size, err := getWindowSize()
	if err == nil {
		err = sendResize(d, size)
		if err != nil {
			log.Fatal(err)
		}
	}

	for {
		select {
		case <- sigCh:
			size, err = getWindowSize()
			if err != nil {
				fmt.Print("error getting window size...\r\n")
				continue
			}
			sendResize(d, size)

		case <-cfg.rsCtx.Done():
			fmt.Print("closing resize watch loop\r\n")
			cfg.closeChan<- true
			return
		}
	}
}

func sendResize(d *webrtc.DataChannel, size *WindowSize) error {
	data, err := json.Marshal(size)
	if err != nil {
		return err
	}

	//data := fmt.Sprintf(
	//	`{"cols":%d,"rows":%d,"colsChanged":%v,"rowsChanged":%v}`,
	//	size.Cols,
	//	size.Rows,
	//	size.ColsChanged,
	//	size.RowsChanged,
	//)

	err = d.SendText(string(data))
	if err != nil {
		return err
	}
	return nil
}

