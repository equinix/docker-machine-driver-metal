// SPDX-License-Identifier: BSD-3-Clause

package metal

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/carmo-evan/strtotime"
	"github.com/docker/machine/libmachine/drivers"
	"github.com/docker/machine/libmachine/log"
	"github.com/docker/machine/libmachine/mcnflag"
	"github.com/docker/machine/libmachine/ssh"
	"github.com/docker/machine/libmachine/state"
	"github.com/packethost/packngo"
	"sigs.k8s.io/yaml"
)

const (
	dockerConfigDir = "/etc/docker"
	consumerToken   = "24e70949af5ecd17fe8e867b335fc88e7de8bd4ad617c0403d8769a376ddea72"
	defaultOS       = "ubuntu_20_04"
	defaultMetro    = "dc"
)

type envSuffix string
type argSuffix string

var (
	// version is set by goreleaser at build time
	version = "devel"

	driverName = "metal"

	envAuthToken       envSuffix = "_AUTH_TOKEN"
	envApiKey          envSuffix = "_API_KEY"
	envProjectID       envSuffix = "_PROJECT_ID"
	envOS              envSuffix = "_OS"
	envFacilityCode    envSuffix = "_FACILITY_CODE"
	envMetroCode       envSuffix = "_METRO_CODE"
	envPlan            envSuffix = "_PLAN"
	envHwId            envSuffix = "_HW_ID"
	envBillingCycle    envSuffix = "_BILLING_CYCLE"
	envUserdata        envSuffix = "_USERDATA"
	envSpotInstance    envSuffix = "_SPOT_INSTANCE"
	envSpotPriceMax    envSuffix = "_SPOT_PRICE_MAX"
	envTerminationTime envSuffix = "_TERMINATION_TIME"
	envUAPrefix        envSuffix = "_UA_PREFIX"

	argAuthToken       argSuffix = "-auth-token"
	argApiKey          argSuffix = "-api-key"
	argProjectID       argSuffix = "-project-id"
	argOS              argSuffix = "-os"
	argFacilityCode    argSuffix = "-facility-code"
	argMetroCode       argSuffix = "-metro-code"
	argPlan            argSuffix = "-plan"
	argHwId            argSuffix = "-hw-reservation-id"
	argBillingCycle    argSuffix = "-billing-cycle"
	argUserdata        argSuffix = "-userdata"
	argSpotInstance    argSuffix = "-spot-instance"
	argSpotPriceMax    argSuffix = "-spot-price-max"
	argTerminationTime argSuffix = "-termination-time"
	argUAPrefix        argSuffix = "-ua-prefix"

	// build time check that the Driver type implements the Driver interface
	_ drivers.Driver = &Driver{}
)

func argPrefix(f argSuffix) string {
	return driverName + string(f)
}

func envPrefix(f envSuffix) string {
	return strings.ToUpper(driverName) + string(f)
}

type Driver struct {
	*drivers.BaseDriver
	ApiKey                  string
	ProjectID               string
	Plan                    string
	HardwareReserverationID string
	Facility                string
	Metro                   string
	OperatingSystem         string
	BillingCycle            string
	DeviceID                string
	UserData                string
	Tags                    []string
	CaCertPath              string
	SSHKeyID                string
	UserDataFile            string
	UserAgentPrefix         string
	SpotInstance            bool
	SpotPriceMax            float64
	TerminationTime         *packngo.Timestamp
}

// NewDriver is a backward compatible Driver factory method.  Using
// new(metal.Driver) is preferred.
func NewDriver(hostName, storePath string) *Driver {
	return &Driver{
		BaseDriver: &drivers.BaseDriver{
			MachineName: hostName,
			StorePath:   storePath,
		},
	}
}

func (d *Driver) GetCreateFlags() []mcnflag.Flag {
	return []mcnflag.Flag{
		mcnflag.StringFlag{
			Name:   argPrefix(argAuthToken),
			Usage:  "Equinix Metal Authentication Token",
			EnvVar: envPrefix(envAuthToken),
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argApiKey),
			Usage:  "Authentication Key (deprecated name, use Auth Token)",
			EnvVar: envPrefix(envApiKey),
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argProjectID),
			Usage:  "Equinix Metal Project Id",
			EnvVar: envPrefix(envProjectID),
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argOS),
			Usage:  "Equinix Metal OS",
			Value:  defaultOS,
			EnvVar: envPrefix(envOS),
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argFacilityCode),
			Usage:  "Equinix Metal facility code",
			EnvVar: envPrefix(envFacilityCode),
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argMetroCode),
			Usage:  fmt.Sprintf("Equinix Metal metro code (%q is used if empty and facility is not set)", defaultMetro),
			EnvVar: envPrefix(envMetroCode),
			// We don't set Value because Facility was previously required and
			// defaulted. Existing configurations with "Facility" should not
			// break. Setting a default metro value would break those
			// configurations.
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argPlan),
			Usage:  "Equinix Metal Server Plan",
			Value:  "c3.small.x86",
			EnvVar: envPrefix(envPlan),
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argHwId),
			Usage:  "Equinix Metal Reserved hardware ID",
			EnvVar: envPrefix(envHwId),
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argBillingCycle),
			Usage:  "Equinix Metal billing cycle, hourly or monthly",
			Value:  "hourly",
			EnvVar: envPrefix(envBillingCycle),
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argUserdata),
			Usage:  "Path to file with cloud-init user-data",
			EnvVar: envPrefix(envUserdata),
		},
		mcnflag.BoolFlag{
			Name:   argPrefix(argSpotInstance),
			Usage:  "Request a Equinix Metal Spot Instance",
			EnvVar: envPrefix(envSpotInstance),
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argSpotPriceMax),
			Usage:  "The maximum Equinix Metal Spot Price",
			EnvVar: envPrefix(envSpotPriceMax),
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argTerminationTime),
			Usage:  "The Equinix Metal Instance Termination Time",
			EnvVar: envPrefix(envTerminationTime),
		},
		mcnflag.StringFlag{
			Name:   argPrefix(argUAPrefix),
			Usage:  fmt.Sprintf("Prefix the User-Agent in Equinix Metal API calls with some 'product/version' %s %s", version, driverName),
			EnvVar: envPrefix(envUAPrefix),
		},
	}
}

func (d *Driver) DriverName() string {
	return driverName
}

func (d *Driver) setConfigFromFile() error {
	configFile := getConfigFile()

	config := metalSnakeConfig{}

	if raw, err := ioutil.ReadFile(configFile); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	} else if jsonErr := yaml.Unmarshal(raw, &config); jsonErr != nil {
		return jsonErr
	}
	d.Plan = config.Plan
	d.ApiKey = config.AuthToken
	if config.Token != "" {
		d.ApiKey = config.Token
	}
	d.Facility = config.Facility
	d.Metro = config.Metro
	d.OperatingSystem = config.OS
	d.ProjectID = config.ProjectID
	return nil
}

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	if err := d.setConfigFromFile(); err != nil {
		return err
	}
	// override config file values with command-line values
	for k, p := range map[string]*string{
		argPrefix(argOS):           &d.OperatingSystem,
		argPrefix(argAuthToken):    &d.ApiKey,
		argPrefix(argProjectID):    &d.ProjectID,
		argPrefix(argMetroCode):    &d.Metro,
		argPrefix(argFacilityCode): &d.Facility,
		argPrefix(argPlan):         &d.Plan,
	} {
		if v := flags.String(k); v != "" {
			*p = v
		}
	}

	oldApiKey := flags.String(argPrefix(argApiKey))

	if d.ApiKey == "" {
		d.ApiKey = oldApiKey

		if d.ApiKey == "" {
			return fmt.Errorf("%s driver requires the --%s option", driverName, argPrefix(argAuthToken))
		}
	} else if oldApiKey != "" {
		log.Warnf("ignoring API Key setting (%s, %s)", argPrefix(argApiKey), envPrefix(envApiKey))
	}

	if strings.Contains(d.OperatingSystem, "coreos") {
		d.SSHUser = "core"
	}
	if strings.Contains(d.OperatingSystem, "rancher") {
		d.SSHUser = "rancher"
	}

	d.BillingCycle = flags.String(argPrefix(argBillingCycle))
	d.UserAgentPrefix = flags.String(argPrefix(argUAPrefix))
	d.UserDataFile = flags.String(argPrefix(argUserdata))
	d.HardwareReserverationID = flags.String(argPrefix(argHwId))
	d.SpotInstance = flags.Bool(argPrefix(argSpotInstance))

	if d.SpotInstance {
		SpotPriceMax := flags.String(argPrefix(argSpotPriceMax))
		if SpotPriceMax == "" {
			d.SpotPriceMax = -1
		} else {
			SpotPriceMax, err := strconv.ParseFloat(SpotPriceMax, 64)
			if err != nil {
				return err
			}
			d.SpotPriceMax = SpotPriceMax
		}

		TerminationTime := flags.String(argPrefix(argTerminationTime))
		if TerminationTime == "" {
			d.TerminationTime = nil
		} else {
			Timestamp, err := strtotime.Parse(TerminationTime, time.Now().Unix())
			if err != nil {
				return err
			}
			if Timestamp <= time.Now().Unix() {
				return fmt.Errorf("--%s cannot be in the past", argPrefix(argTerminationTime))
			}
			d.TerminationTime = &packngo.Timestamp{Time: time.Unix(Timestamp, 0)}
		}
	}

	if d.ProjectID == "" {
		return fmt.Errorf("%s driver requires the --%s option", driverName, argPrefix(argProjectID))
	}

	return nil
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

func (d *Driver) PreCreateCheck() error {
	if d.UserDataFile != "" {
		if _, err := os.Stat(d.UserDataFile); os.IsNotExist(err) {
			return fmt.Errorf("user-data file %s could not be found", d.UserDataFile)
		}
	}

	flavors, err := d.getOsFlavors()
	if err != nil {
		return err
	}
	if !stringInSlice(d.OperatingSystem, flavors) {
		return fmt.Errorf("specified --%s not one of %v", argPrefix(argOS), strings.Join(flavors, ", "))
	}

	if d.Metro == "" && d.Facility == "" {
		d.Metro = defaultMetro
	}

	if d.Metro != "" && d.Facility != "" {
		return fmt.Errorf("facility and metro can not be used together")
	}

	client := d.getClient()

	if d.Metro != "" {
		return validateMetro(client, d.Metro)
	}

	return validateFacility(client, d.Facility)
}

func (d *Driver) Create() error {
	var userdata string
	if d.UserDataFile != "" {
		buf, err := ioutil.ReadFile(d.UserDataFile)
		if err != nil {
			return err
		}
		userdata = string(buf)
	}

	log.Info("Creating SSH key...")

	key, err := d.createSSHKey()
	if err != nil {
		return err
	}

	d.SSHKeyID = key.ID

	hardwareReservationId := ""
	//check if hardware reservation requested
	if d.HardwareReserverationID != "" {
		hardwareReservationId = d.HardwareReserverationID
	}

	client := d.getClient()
	createRequest := &packngo.DeviceCreateRequest{
		Hostname:              d.MachineName,
		Plan:                  d.Plan,
		HardwareReservationID: hardwareReservationId,
		Metro:                 d.Metro,
		OS:                    d.OperatingSystem,
		BillingCycle:          d.BillingCycle,
		ProjectID:             d.ProjectID,
		UserData:              userdata,
		Tags:                  d.Tags,
		SpotInstance:          d.SpotInstance,
		SpotPriceMax:          d.SpotPriceMax,
		TerminationTime:       d.TerminationTime,
	}

	if d.Facility != "" {
		createRequest.Facility = []string{d.Facility}
	}

	log.Info("Provisioning Equinix Metal server...")
	newDevice, _, err := client.Devices.Create(createRequest)
	if err != nil {
		log.Errorf("device could not be created: %s", err)

		//cleanup ssh keys if device failed
		if _, err := client.SSHKeys.Delete(d.SSHKeyID); ignoreStatusCodes(err, http.StatusForbidden, http.StatusNotFound) != nil {
			log.Errorf("ssh-key could not be deleted: %s", err)
			return err
		}
		return err
	}
	t0 := time.Now()

	d.DeviceID = newDevice.ID

	for {
		newDevice, _, err = client.Devices.Get(d.DeviceID, nil)
		if err != nil {
			return err
		}

		for _, ip := range newDevice.Network {
			if ip.Public && ip.AddressFamily == 4 {
				d.IPAddress = ip.Address
			}
		}

		if d.IPAddress != "" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	log.Infof("Created device ID %s, IP address %s",
		newDevice.ID,
		d.IPAddress)

	log.Info("Waiting for Provisioning...")
	stage := float32(0)
	for {
		newDevice, _, err = client.Devices.Get(d.DeviceID, nil)
		if err != nil {
			return err
		}
		if newDevice.State == "provisioning" && stage != newDevice.ProvisionPer {
			stage = newDevice.ProvisionPer
			log.Debugf("Provisioning %v%% complete", newDevice.ProvisionPer)
		}
		if newDevice.State == "active" {
			log.Debugf("Device State: %s", newDevice.State)
			break
		}
		time.Sleep(10 * time.Second)
	}

	log.Debugf("Provision time: %v.", time.Since(t0))

	log.Debug("Waiting for SSH...")
	if err := drivers.WaitForSSH(d); err != nil {
		return err
	}

	return nil
}

func (d *Driver) createSSHKey() (*packngo.SSHKey, error) {
	sshKeyPath := d.GetSSHKeyPath()
	log.Debugf("Writing SSH Key to %s", sshKeyPath)

	if err := ssh.GenerateSSHKey(sshKeyPath); err != nil {
		return nil, err
	}

	publicKey, err := ioutil.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		return nil, err
	}

	createRequest := &packngo.SSHKeyCreateRequest{
		Label: fmt.Sprintf("docker machine: %s", d.MachineName),
		Key:   string(publicKey),
	}

	key, _, err := d.getClient().SSHKeys.Create(createRequest)
	if err != nil {
		return key, err
	}

	return key, nil
}

func (d *Driver) GetURL() (string, error) {
	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tcp://%s:2376", ip), nil
}

func (d *Driver) GetIP() (string, error) {
	if d.IPAddress == "" {
		return "", fmt.Errorf("IP address is not set")
	}
	return d.IPAddress, nil
}

func (d *Driver) GetState() (state.State, error) {
	device, _, err := d.getClient().Devices.Get(d.DeviceID, nil)
	if err != nil {
		return state.Error, err
	}

	switch device.State {
	case "queued", "provisioning", "powering_on":
		return state.Starting, nil
	case "active":
		return state.Running, nil
	case "powering_off":
		return state.Stopping, nil
	case "inactive":
		return state.Stopped, nil
	}
	return state.None, nil
}

func (d *Driver) Start() error {
	_, err := d.getClient().Devices.PowerOn(d.DeviceID)
	return err
}

func (d *Driver) Stop() error {
	_, err := d.getClient().Devices.PowerOff(d.DeviceID)
	return err
}

func ignoreStatusCodes(err error, codes ...int) error {
	e, ok := err.(*packngo.ErrorResponse)
	if !ok || e.Response == nil {
		return err
	}

	for _, c := range codes {
		if e.Response.StatusCode == c {
			return nil
		}
	}
	return err
}

func (d *Driver) Remove() error {
	client := d.getClient()
	if _, err := client.SSHKeys.Delete(d.SSHKeyID); ignoreStatusCodes(err, http.StatusForbidden, http.StatusNotFound) != nil {
		return err
	}

	_, err := client.Devices.Delete(d.DeviceID, false)
	return ignoreStatusCodes(err, http.StatusForbidden, http.StatusNotFound)
}

func (d *Driver) Restart() error {
	_, err := d.getClient().Devices.Reboot(d.DeviceID)
	return err
}

func (d *Driver) Kill() error {
	_, err := d.getClient().Devices.PowerOff(d.DeviceID)
	return err
}

func (d *Driver) GetDockerConfigDir() string {
	return dockerConfigDir
}

func (d *Driver) getClient() *packngo.Client {
	client := packngo.NewClientWithAuth(consumerToken, d.ApiKey, nil)
	userAgent := fmt.Sprintf("docker-machine-driver-%s/%s %s", d.DriverName(), version, client.UserAgent)

	if len(d.UserAgentPrefix) > 0 {
		userAgent = fmt.Sprintf("%s %s", d.UserAgentPrefix, userAgent)
	}

	client.UserAgent = userAgent
	return client
}

func (d *Driver) getOsFlavors() ([]string, error) {
	operatingSystems, _, err := d.getClient().OperatingSystems.List()
	if err != nil {
		return nil, err
	}

	supportedDistros := []string{
		"centos",
		"coreos",
		"debian",
		"opensuse",
		"rancher",
		"ubuntu",
	}
	flavors := make([]string, 0, len(operatingSystems))
	for _, flavor := range operatingSystems {
		if stringInSlice(flavor.Distro, supportedDistros) {
			flavors = append(flavors, flavor.Slug)
		}
	}
	return flavors, nil
}

func validateFacility(client *packngo.Client, facility string) error {
	if facility == "any" {
		return nil
	}

	facilities, _, err := client.Facilities.List(nil)
	if err != nil {
		return err
	}
	for _, f := range facilities {
		if f.Code == facility {
			return nil
		}
	}

	return fmt.Errorf("%s requires a valid facility", driverName)
}

func validateMetro(client *packngo.Client, metro string) error {
	metros, _, err := client.Metros.List(nil)
	if err != nil {
		return err
	}
	for _, m := range metros {
		if m.Code == metro {
			return nil
		}
	}

	return fmt.Errorf("%s requires a valid metro", driverName)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
