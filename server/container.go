package server

import (
	"errors"
	"html/template"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write the file to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Poll file for changes with this period.
	flushPeriod = 1 * time.Second
)

const homeHTML = `<!DOCTYPE html>
<html lang="en">
    <head>
        <title>WebSocket Example</title>
    </head>
    <body>
        <pre id="data">{{.Data}}</pre>
        <script type="text/javascript">
            (function() {
                
			})();
			var data = document.getElementById("data");
                var conn = new WebSocket("ws://{{.Host}}/ws");
                conn.onclose = function(evt) {
                    data.textContent = 'Connection closed';
                }
                conn.onmessage = function(evt) {
                    data.textContent = evt.data;
                }
        </script>
    </body>
</html>
`

var (
	addr      = ":8080"
	homeTempl = template.Must(template.New("").Parse(homeHTML))
	upgrader  = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// ErrNoReader ...
var ErrNoReader = errors.New("no reader")

// WebSocketContainer ...
type WebSocketContainer struct {
	Conn            *websocket.Conn
	Reader          io.Reader
	Mux             sync.RWMutex
	handleAddReader func() io.Reader
}

// Container ...
var Container *WebSocketContainer

// New return a Container entity.
// If reader is nil, it must have handle whitch can create a io.Reader.
// If handle is nil, reader should not be nil. If it is, the server will throw panic.
func New(reader io.Reader, handle func() io.Reader) (*WebSocketContainer, error) {
	Container = &WebSocketContainer{
		Reader:          reader,
		handleAddReader: handle,
	}

	return Container, nil
}

// Run run a server on the specify address, or default address :8080
func (c *WebSocketContainer) Run(address string) {
	if address != "" {
		addr = address
	}
	log.Printf("Startint the server...")
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	log.Printf("Listening %v\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
}

func (c *WebSocketContainer) connect(ws *websocket.Conn) error {
	c.Mux.Lock()
	defer c.Mux.Unlock()

	c.Conn = ws
	return nil
}

func (c *WebSocketContainer) newReader(reader io.Reader) error {
	c.Mux.Lock()
	defer c.Mux.Unlock()

	c.Reader = reader
	return nil
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}

	err = Container.connect(ws)
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("Websocket connecting...")
	go writer(ws)
	reader(ws)
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var v = struct {
		Host string
		Data string
	}{
		r.Host,
		string("Loading..."),
	}
	homeTempl.Execute(w, &v)
}

func reader(ws *websocket.Conn) {
	defer ws.Close()
	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Websocket read stop error: %v\n", err)
			break
		}
	}
}

func writer(ws *websocket.Conn) {
	lastError := ""
	pingTicker := time.NewTicker(pingPeriod)
	flushTicker := time.NewTicker(flushPeriod)
	defer func() {
		pingTicker.Stop()
		flushTicker.Stop()
		ws.Close()
		log.Printf("Websocket closed.")
	}()
	for {
		select {
		case <-flushTicker.C:
			var errString []byte
			var err error

			err = readData(Container.Reader, Container.Conn)
			if err == ErrNoReader {
				if Container.handleAddReader == nil {
					panic("Do not have the function to get a new io.Reader.")
				}
				err = Container.newReader(Container.handleAddReader())
				if err != nil {
					log.Printf("add reader error: %v\n", err)
				}
			} else if err != nil {
				if s := err.Error(); s != lastError {
					lastError = s
					errString = []byte(lastError)
				}
			} else {
				lastError = ""
			}

			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if len(errString) != 0 {
				if err := ws.WriteMessage(websocket.TextMessage, errString); err != nil {
					log.Printf("Websocket error message: %v\n", err)
					return
				}
			}
		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Printf("Websocket ping message error: %v\n", err)
				return
			}
			log.Printf("Websocket ping\n")
		}
	}
}

func readData(r io.Reader, ws *websocket.Conn) error {
	if r == nil {
		log.Printf("Error: The reader is not inited.")
		return ErrNoReader
	}
	w, err := ws.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	if _, err := io.Copy(w, r); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}
