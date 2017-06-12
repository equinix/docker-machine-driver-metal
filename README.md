# docker-machine-driver-packet
Packet bare-metal cloud driver for Docker Machine called 

> Driver name: `packet`

### Usage

You can provision bare-metal hosts once you have built and installed the docker-machine driver. The binary will be placed in your `$PATH` directory.

Test that the installation worked by typing in:

```
$ docker-machine create --driver packet
```

### Building

Pre-reqs: `docker-machine` and `make`

* Install the Golang SDK [https://golang.org/dl/](https://golang.org/dl/)

* Download the source-code with `go get -u github.com/packethost/docker-machine-driver-packet`

* Build and install the driver:

```
$ cd $GOPATH/github.com/packethost/docker-machine-driver-packet
$ make 
$ sudo make install
```

Now you will now be able to specify a `-driver` of `packet` to `docker-machine` commands.
