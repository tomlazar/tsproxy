# setup builder image
FROM --platform=$BUILDPLATFORM golang:1.19-alpine as builder 
WORKDIR /src

# copy and download mod file
COPY go.mod .
COPY go.sum .
RUN go mod download

# build app
COPY ./tsproxy.go .

ARG TARGETOS TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH  go build -o tsproxy .

# # Now copy it into our base image.
FROM --platform=$BUILDPLATFORM alpine
LABEL org.opencontainers.image.source https://github.com/tomlazar/tsproxy
WORKDIR /
COPY --from=builder /src/tsproxy /tsproxy
CMD ["/tsproxy"]