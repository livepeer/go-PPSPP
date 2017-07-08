# go-PPSPP

go-PPSPP is a Go implementation of the Peer-to-Peer Streaming Peer Protocol ([rfc7574](https://tools.ietf.org/html/rfc7574)).

The PPSPP protocol serves as a transport layer for video live streaming.  It is format-agnostic (can be used to transport any video format), and network protocol-agnostic (should be able to talk to any network protocol like tcp, udp, webrtc, etc).

This Go implementation is intended to be used as a library for go projects that want to use PPSPP for live video streaming.

### Installation

`go get github.com/livepeer/go-PPSPP`

### Usage

Import the core package into your go project:

`import "github.com/livepeer/go-PPSPP/core"`

Take a look at [simple.go](examples/simple/simple.go) for an example of how to use go-PPSPP.

You can run the simple example like this:

`go run examples/simple/simple.go`

You should see a lot of logger spew and then it should terminate without error.

More examples are on the way, and examples will likely change significantly as the interfaces are still a work in progress.

### Contributing

All contributions and bug fixes are welcome as pull requests back into the repo.

To run all tests, from the core directory, run:

`go test -timeout 30s`

Please report protocol bugs big and small by [opening an issue](https://github.com/livepeer/go-PPSPP/issues/new). No possible bug report is too small.

### Credits

This project was created and is maintained by [Livepeer](https://livepeer.org) and others. See the complete list of [contributors](https://github.com/livepeer/go-PPSPP/contributors).

### License

This project is [licensed under the terms of the MIT license](LICENSE).
