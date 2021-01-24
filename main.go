// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"github.com/docker/machine/libmachine/drivers/plugin"
	metal "github.com/packethost/docker-machine-driver-packet/pkg/drivers/equinix-metal"
)

func main() {
	plugin.RegisterDriver(new(metal.Driver))
}
