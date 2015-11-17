package main

import (
	"github.com/docker/machine/libmachine/drivers/plugin"
	"github.com/packethost/docker-machine-driver-packet"
)

func main() {
	plugin.RegisterDriver(new(packet.Driver))
}
