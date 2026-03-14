package main

import (
	//"encoding/json"
	"fmt"
	"time"
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
	fmt.Printf("%v \r\n", *size)
	if err == nil {
		sendResize(d, size)
	}

	for {
		select {
		case <- sigCh:
			size, err = getWindowSize()
			if err != nil {
				continue
			}
			sendResize(d, size)

		case <-cfg.rsCtx.Done():
			return
		}
	}
}

func sendResize(d *webrtc.DataChannel, size *WindowSize) error {
	//data, err := json.Marshal(size)
	//if err != nil {
	//	return err
	//}
	time.Sleep(100 * time.Millisecond)

	data := fmt.Sprintf(
		`{"data_type":"string","data":"{\"cols\":%d,\"rows\":%d,\"colsChanged\":%v,\"rowsChanged\":%v}"`,
		117,
		size.Rows,
		size.ColsChanged,
		size.RowsChanged,
	)

	fmt.Printf("%s\r\n", data)

	err := d.SendText(data)
	if err != nil {
		return err
	}
	return nil
}

