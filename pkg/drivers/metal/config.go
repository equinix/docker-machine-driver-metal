// SPDX-License-Identifier: BSD-3-Clause

package metal

import (
	"os"
	"path"
	"runtime"
)

type metalSnakeConfig struct {
	Token     string `json:"token,omitempty"`
	AuthToken string `json:"auth-token,omitempty"`
	Facility  string `json:"facility,omitempty"`
	Metro     string `json:"metro,omitempty"`
	OS        string `json:"operating-system,omitempty"`
	Plan      string `json:"plan,omitempty"`
	ProjectID string `json:"project-id,omitempty"`
}

func getConfigFile() string {
	configFile := os.Getenv("METAL_CONFIG")
	if configFile != "" {
		return configFile
	}

	return path.Join(userHomeDir(), "/.config/equinix/metal.yaml")
}

func userHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}
