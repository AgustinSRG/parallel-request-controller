# Parallel Request Controller

Simple backend component to control parallel requests.

The main use case for this component is to impose limits on parallel requests when using multiple web servers for horizontal scaling.

## Compilation

In order to compile the project, navigate to the [server](./server/) folder and run the [Golang compiler](https://go.dev/doc/install):

```
go build .
```

The build command will create a binary in the current directory, called `server`, or `server.exe` if you are using Windows.

## Docker Image

You can find the docker image for this project available in Docker Hub: [https://hub.docker.com/r/asanrom/parallel-request-controller](https://hub.docker.com/r/asanrom/parallel-request-controller)

To pull it type:

```
docker pull asanrom/parallel-request-controller
```

## Server configuration

You can configure the server using environment variables. You can set up a `.env` file in the current working directory in order to set them in an easy way.

Here is a list with all the available configuration variables for the server.

### General

| Variable       | Description                                                                 |
| -------------- | --------------------------------------------------------------------------- |
| `PORT`         | The listening port for the server. By default: `8080`                       |
| `BIND_ADDRESS` | Bind address for the server. By default it binds to all network interfaces. |

### TLS

| Variable          | Description                                                                                                           |
| ----------------- | --------------------------------------------------------------------------------------------------------------------- |
| `TLS_ENABLED`     | Can be `YES` or `NO`. If `YES`, TLS will be enabled for the server, and client must connect with the `wss:` protocol. |
| `TLS_CERTIFICATE` | Path to the certificate file to load (PEM format).                                                                    |
| `TLS_PRIVATE_KEY` | Path to the private key file to load (PEM format).                                                                    |

### Logs

| Variable    | Description                                                                                             |
| ----------- | ------------------------------------------------------------------------------------------------------- |
| `LOG_INFO`  | Can be `YES` or `NO`. If `YES`, it will log information messages to the standard output. Default: `YES` |
| `LOG_DEBUG` | Bind address for the server. By default it binds to all network interfaces.                             |

## Clients

- [Go client](./client/)
- [Javascript client](./client-js/)

## Documentation

- [Protocol](./PROTOCOL.md)

## License

This project is under the [MIT License](./LICENSE).
