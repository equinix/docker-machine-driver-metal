package metal

import (
	"os"
	"testing"

	"github.com/docker/machine/libmachine/drivers"
	"github.com/stretchr/testify/assert"
)

func TestSetConfigFromFlags(t *testing.T) {
	driver := NewDriver("", "")
	configPath := os.Getenv("METAL_CONFIG")
	os.Setenv("METAL_CONFIG", "/does-not-exist")
	checkFlags := &drivers.CheckDriverOptions{
		FlagsValues: map[string]interface{}{
			"metal-api-key":    "APIKEY",
			"metal-project-id": "PROJECT",
		},
		CreateFlags: driver.GetCreateFlags(),
	}

	err := driver.SetConfigFromFlags(checkFlags)
	os.Setenv("METAL_CONFIG", configPath)
	assert.NoError(t, err)
	assert.Empty(t, checkFlags.InvalidFlags)
}
