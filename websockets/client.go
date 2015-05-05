package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "localhost:8080", "address to connect to")

func main() {
	flag.Parse()

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter your name: ")
	name, err := reader.ReadSlice('\n')
	if err != nil {
		fmt.Println(err)
		return
	}

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(fmt.Sprintf("ws://%s/ws?name=%s", *addr, name[:len(name)-1]), nil)
	if err != nil {
		panic(err)
	}

	wsClose := make(chan bool)
	go readMessages(conn, wsClose)
	go writeMessages(conn, wsClose)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGPIPE)
	select {
	case s := <-sigChan:
		fmt.Printf("%s signal received, stopping process...\n", s)
	case <-wsClose:
		fmt.Println("The websocket closed")
	}
}

func readMessages(conn *websocket.Conn, ch chan bool) {
	defer func() {
		conn.Close()
		ch <- true
	}()
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		fmt.Println(string(message))
	}
}

func writeMessages(conn *websocket.Conn, ch chan bool) {
	defer func() {
		conn.Close()
		ch <- true
	}()
	reader := bufio.NewReader(os.Stdin)
	for {
		text, err := reader.ReadSlice('\n')
		if err != nil {
			fmt.Println(err)
			break
		}
		msg := text[:len(text)-1]
		if len(msg) > 0 {
			conn.WriteMessage(1, msg)
		}
	}
}
