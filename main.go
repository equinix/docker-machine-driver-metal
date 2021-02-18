// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"github.com/docker/machine/libmachine/drivers/plugin"
	metal "github.com/equinix/docker-machine-driver-metal/pkg/drivers/metal"
)

func main() {
	plugin.RegisterDriver(new(metal.Driver))
}
