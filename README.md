# Web Console

>It can print data on web by websocket. But only have one server entity.

## Install

    go get -u github.com/wangxianzhuo/web-console

## Usage

```golang
package main

import (
    "bytes"
    "time"

    "github.com/wangxianzhuo/web-console/server"
)

func main() {
    var buffer bytes.Buffer
    server, _ := server.New(&buffer, nil)
    go func() {
        for {
            time.Sleep(1 * time.Second)
            // It can print newest time on web.
            buffer.Write([]byte(time.Now().String()))
        }
    }()
    server.Run("")
}
```

```shell
go run main.go
```

> Visiting `http://localhost:8080/` can see the newest time.
