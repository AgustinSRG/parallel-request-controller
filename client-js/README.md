# Javascript client for Parallel request controller

This is the Javascript client library to connect to a [Parallel Request Controller Server](../server/).

## Installation

In order to install the library into your project, run the following command:

```sh
npm install @asanrom/parallel-request-controller
```

## Usage

In order to connect to the PRC server, you must instantiate `PRCClient` and call the `connect` method.

Example:

```ts
import { PRCClient } from "@asanrom/parallel-request-controller";

const prcClient = new PRCClient({
  // PRC server base URL
  url: process.env.SERVER_URL || "ws://localhost:8080",
  // Authentication token
  authToken: process.env.AUTH_TOKEN || "",
  onServerError: (code, reason) => {
    // Called on a PRC server error
    console.error("[SERVER ERROR] " + code + ": " + reason);
  },
  onConnectionError: (err) => {
    // Called on a connection error
    console.error(err);
  },
});

prcClient.connect(); // Call connect to open the Websocket connections
```

Once you have instantiated `PRCClient`, you can call the `startRequest` method to indicate a request starting. It will return an object with a property `limited` that you can check in order to decide to drop the request. After the request is done, you must call the `endRequest` method of the client.

Example:

```ts
const MAX_PARALLEL_REQUESTS = 5;

async function handleRequest(request, response) {
  let prcRef;
  try {
    prcRef = await prcClient.startRequest(
      request.requestType,
      MAX_PARALLEL_REQUESTS
    );
  } catch (ex) {
    console.error(ex);
    response.status(500);
    response.send("Internal server error");
    return;
  }

  if (prcRef.limited) {
    // Request limit reached
    response.status(429);
    response.send("Request limit reached.");
    return;
  }

  try {
    // Handle request normally
    // ...
  } catch (ex) {
    console.error(ex);
    response.status(500);
    response.send("Internal server error");
  }

  prcClient.endRequest(prcRef);
}
```

## Documentation

- https://agustinsrg.github.io/parallel-request-controller/client-js/docs

## Building source code

In order to build the library from its source code, run the following commands:

```sh
npm install
npm run build
```

## Testing

In order to test the library, first, make sure to start a [Parallel Request Controller Server](../server/). Also, set the following env variables:

| Variable     | Description                                |
| ------------ | ------------------------------------------ |
| `SERVER_URL` | Server URL. Default: `ws://localhost:8080` |
| `AUTH_TOKEN` | Authentication token                       |

Then, run:

```sh
npm test
```
