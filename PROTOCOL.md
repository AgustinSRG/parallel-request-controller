# Protocol for Parallel Request Controller

The clients will connect to the parallel request controller server via **websocket**, at the `/ws/{AUTH_TOKEN}` path. REplace `{AUTH_TOKEN}` with the authentication token that was set in the server configuration.

```
ws(s)://{HOST}:{PORT}/ws/{AUTH_TOKEN}
```

## Message format

The messages are UTF-8 encoded strings, with parts split by line breaks (\n):
 
  - The first line is the message type (upper case string)
  - After it, the message can have an arbitrary number of parameters. Each parameter has a name, followed by a colon and it's value. Parameter names are case-insensitive.
  - Optionally, after the arguments, it can be an empty line, followed by the body of the message (an arbitrary string).

```
MESSAGE-TYPE
Argument: value

{body}
```

## Message types

Here is the full list of message types, including their purpose and full structure explained.

### Heartbeat

After authenticated, to keep the connection active, both, the client and the server will exchange heartbeat messages every 30 seconds.

If the server or the client do not receive a heartbeat message during 1 minute, the connection may be closed due to inactivity.

The message does not take any arguments or body.

```
HEARTBEAT
```

### Start-Request

In order to indicate a request is about to start, the client will send a `START-REQUEST` message.

The required arguments are:

 - `Request-ID` - An unique id for the request. It will be used to track the response and to indicate the ending of the request.
 - `Request-Type` - An arbitrary string indicating the type of request.
 - `Request-Limit` - An integer indicating the max number of request of `Request-Type` that can be handled in parallel.

Example:

```
START-REQUEST
Request-ID: 0001
Request-Type: download-file0001-user0001
Request-Limit: 1
```

### Start-Request-Ack

When the server receives a `START-REQUEST` message, it will respond to the client with a `START-REQUEST-ACK` message, indicating if the request limit was reached.

The required arguments are:

 - `Request-ID` - The unique id for the request.
 - `Request-Limit-Reached` - Can be `TRUE` or `FALSE`. It it is `TRUE`, it means the request should be rejected, since the request limit was reached.

Example:

```
START-REQUEST
Request-ID: 0001
Request-Limit-Reached: FALSE
```

### End-Request

In order to indicate the ending of a request, the client will send a `END-REQUEST` message.

The required arguments are:

 - `Request-ID` - The unique id for the request.

Example:

```
END-REQUEST
Request-ID: 0001
```

### Get-Request-Count

If the client wants to know the current parallel request count for a type, it will send a `GET-REQUEST-COUNT` message.

The required arguments are:

 - `Request-Type` - An arbitrary string indicating the type of request.

Example:

```
GET-REQUEST-COUNT
Request-Type: download-file0001-user0001
```

### Request-Count

When the server receives a `GET-REQUEST-COUNT` message, it will respond with a `REQUEST-COUNT` message.

The required arguments are:

 - `Request-Type` - An arbitrary string indicating the type of request.
 - `Request-Count` - Number of requests being handled in parallel at the moment.ç

### Error

If an error happens, the server will send an `ERROR` message, with the details of the error in the arguments.

```
ERROR
Error-Code: EXAMPLE_CODE
Error-Message: Example Error message
```

## Crashing and disconnecting

The server will keep track of the requests for each websocket connection. If the websocket connection is closed, every single pending request will be considered ended, since this can happen due to the client crashing.

If the controller crashes, every pending request will be considered ended.
