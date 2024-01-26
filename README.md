# Cache

This package provides a simple set of definitions for a cache interface and
some implementations.

## Installation

```bash
go get github.com/tangelo-labs/go-cache
```

## Usage

```go
package main

import (
	"context"

	"github.com/tangelo-labs/go-cache/provider/mcache"
)

func main() {
    cache, err := mcache.NewLRU[int](100)
    if err != nil {
        panic(err)
    }
    
    if err := cache.Put(context.TODO(), "test", 27); err != nil {
        panic(err)
    }
}
```