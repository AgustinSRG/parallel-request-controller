# Go client library for: Parallel Request Controller

This is a client library for the [Go](https://go.dev/) programming language in order to connect to the [Parallel Request Controller Server](../server/).

## Usage

In order to fetch the library, type:

```sh
go get github.com/AgustinSRG/parallel-request-controller/client
```

Example usage:

```go
package main

import (
    prc_client "github.com/AgustinSRG/parallel-request-controller/client"
)

const MAX_PARALLEL_REQUESTS = 5;

func main() {
    // Create a client for the Parallel Request Controller server
    prcCli := prc_client.NewClient(&ClientConfig{
        Url: "ws://localhost:8080",
        AuthToken: "change_me",
    })

    // For this example, we use a mock server to illustrate the request handling
    server := createServerSomehow(func (req *Request) *Response {
        // We call StartRequest in order to ensure the request limit was not reached 
        prcRef, limited, err := prc_client.StartRequest(request.req_type, MAX_PARALLEL_REQUESTS)

        if err != nil {
            // Handle error
            return &Response{
                Status: 500,
            }
        }

        if limited {
            // Request limit reached
            return &Response{
                Status: 429,
            }
        }

        defer prcRef.End() // When the request finished, we must call End()

        // Compute request...
        // ...
    })

    server.listen()
}
```

## Documentation

- https://pkg.go.dev/github.com/AgustinSRG/parallel-request-controller/client
