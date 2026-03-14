package main

import (
	//"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

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
		RowsChanged: false,
	}
	return ws, nil
}

func (cfg *apiConfig) watchResize(d io.Writer) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	size, err := getWindowSize()
	fmt.Printf("%v \r\n", size)
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

func sendResize(d io.Writer, size *WindowSize) error {
	//data, err := json.Marshal(size)
	//if err != nil {
	//	return err
	//}
	data := fmt.Sprintf(
		`{\"cols\":%d,\"rows\":%d,\"colsChanged\":%v,\"rowsChanged\":%v}`,
		size.Cols,
		size.Rows,
		size.ColsChanged,
		size.RowsChanged,
	)

	fmt.Printf("%s\r\n", data)

	_, err := d.Write([]byte(data))
	if err != nil {
		return err
	}
	return nil
}

