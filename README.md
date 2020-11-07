# eset
[![Go Report Card](https://goreportcard.com/badge/github.com/ichxxx/eset)](https://goreportcard.com/report/github.com/ichxxx/eset)

Expirable, goroutine safe set type for golang


## Installation
`go get github.com/ichxxx/eset`

## Usage
```go
import (
    "fmt"
    "time"
    
    "github.com/ichxxx/eset"
)

func main() {
    es := eset.New()
    
    for i := 0; i < 3; i++ {
        es.Add("foo")
    }
    
    es.AddWithExpire("bar", time.Second * 2)
    
    time.Sleep(time.Second * 2)
    
    fmt.Println(es.GetAll())
}
```
