# quictun

quictun is a simple hidden tunnel based on the QUIC protocol.

This repository contains a proof-of-concept implementation of [quictun](https://github.com/julienschmidt/quictun-thesis).
Its purpose is to demonstrate that quictun clients and servers can be implemented with minimal effort on top of an existing QUIC and HTTP/2 over QUIC implementation.
The implementation uses the [quic-go](https://github.com/lucas-clemente/quic-go) QUIC implementation as a basis.

Note that while quictun is meant to be implemented on top of [IETF QUIC](https://datatracker.ietf.org/wg/quic/about/), this proof-of-concept implementation uses Google QUIC instead, as at the time of development no usable implementation of the (still work-in-progress) IETF version exists. Due to the limitations of the underlying QUIC implementation, this quictun implementation is neither meant for production usage, nor for performance evaluation of the approach.


## Overview

`h2quic` is a fork of [github.com/lucas-clemente/quic-go/h2quic](https://github.com/lucas-clemente/quic-go/tree/master/h2quic). It adds the upgrade mechanism to the HTTP/2 over QUIC (h2quic) implementation. The fork can be used as a drop-in replacement for the upstream package to add support for quictun.

`cmd/quictun_client` contains a very minimal client example. Actual clients MUST take care to be indistinguishable from an legitimate HTTP/2 over QUIC client, which a censor is unwilling to block, at the wire level. This could be achieved e.g. by reusing the net stack of a QUIC-capable web browser.

`cmd/quictun_server` likewise contains a minimal server example. Note that this example server is easily fingerprintable and thus blockable. 


## Installation

```sh
go get -u quictun
```


