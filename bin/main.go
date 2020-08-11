// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"github.com/docker/machine/libmachine/drivers/plugin"
	packet "github.com/packethost/docker-machine-driver-packet"
)

func main() {
	plugin.RegisterDriver(new(packet.Driver))
}
