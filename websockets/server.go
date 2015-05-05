package main

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

func main() {
	h := newHub()
	go h.run()

	http.Handle("/ws", wsHandler{h: h})
	http.Handle("/", http.FileServer(http.Dir(".")))
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

type hub struct {
	connections map[*connection]bool
	broadcast   chan message
	register    chan *connection
	unregister  chan *connection
}

func newHub() *hub {
	return &hub{
		connections: make(map[*connection]bool),
		broadcast:   make(chan message),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
	}
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
		case c := <-h.unregister:
			if _, ok := h.connections[c]; ok {
				delete(h.connections, c)
				close(c.send)
			}
		case m := <-h.broadcast:
			for c := range h.connections {
				select {
				case c.send <- m:
				default:
					delete(h.connections, c)
					close(c.send)
				}
			}
		}
	}
}

type connection struct {
	name string
	mode string
	ws   *websocket.Conn
	send chan message
	h    *hub
}

func (c *connection) reader() {
	for {
		_, m, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		c.h.broadcast <- message{from: c.name, msg: m}
	}
	c.ws.Close()
}

func (c *connection) writer() {
	for m := range c.send {
		var msg string
		if bytes.Equal(m.msg, welcomeMsg) {
			if c.name == m.from {
				msg = "You have connected."
			} else {
				msg = fmt.Sprintf("%s has connected to this session.", m.from)
			}
		} else {
			if c.mode == "cli" {
				msg = fmt.Sprintf("\x1b[4m%s\x1b[0m: %s", m.from, m.msg)
			} else {
				msg = fmt.Sprintf("<u>%s</u>: %s", m.from, bytes.Replace(m.msg, []byte("\n"), []byte("<br>"), -1))
			}
		}
		if len(msg) > 0 {
			err := c.ws.WriteMessage(1, []byte(msg))
			if err != nil {
				break
			}
		}
	}
	c.ws.Close()
}

type message struct {
	from string
	msg  []byte
}

var welcomeMsg = []byte("--connected--")

var upgrader = &websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// We'll allow all origin requests for this.
		return true
	},
}

type wsHandler struct {
	h *hub
}

func (wsh wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		name = "unknown user"
	}
	c := &connection{name: name, send: make(chan message, 256), ws: ws, h: wsh.h}
	if ua := r.Header.Get("User-Agent"); ua != "" {
		c.mode = "browser"
	} else {
		c.mode = "cli"
	}
	c.h.register <- c
	defer func() { c.h.unregister <- c }()
	go c.writer()
	wsh.h.broadcast <- message{from: name, msg: welcomeMsg}
	c.reader()
}
