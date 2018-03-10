package main

import (
	"bytes"
	"time"

	"github.com/wangxianzhuo/web-console/server"
)

// Execute `go run main.go`.Visiting http://localhost:8080 can see the newest time auto updating.
func main() {
	var buffer bytes.Buffer
	server, _ := server.New(&buffer, nil)
	go func() {
		for {
			time.Sleep(1 * time.Second)
			buffer.Write([]byte(time.Now().String()))
		}
	}()
	server.Run("")
}
