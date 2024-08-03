## Build golang modules ###

FROM golang:latest

WORKDIR /root

ADD /server /root/server

WORKDIR /root/server

# Compile

RUN go get .

RUN go build .

# Prepare runner

FROM alpine as runner

# Add gcompat

RUN apk add gcompat

# Copy binaries

COPY --from=0 /root/server/server /bin/server

# Expose ports

EXPOSE 8080

# Entrypoint

ENTRYPOINT ["/bin/server"]
