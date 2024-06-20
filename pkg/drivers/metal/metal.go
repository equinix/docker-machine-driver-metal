// SPDX-License-Identifier: BSD-3-Clause

package metal

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
	"github.com/equinix/equinix-sdk-go/services/metalv1"
	metal "github.com/equinix/equinix-sdk-go/services/metalv1"
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
	SpotPriceMax            float32
	TerminationTime         *time.Time
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

	if raw, err := os.ReadFile(configFile); err != nil {
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
			SpotPriceMax, err := strconv.ParseFloat(SpotPriceMax, 32)
			if err != nil {
				return err
			}
			d.SpotPriceMax = float32(SpotPriceMax)
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
			t := time.Unix(Timestamp, 0)
			d.TerminationTime = &t
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

type DeviceCreator interface {
	SetPlan(string)
	SetOperatingSystem(string)
	SetHostname(string)
	SetUserdata(string)
	SetTags([]string)
	SetHardwareReservationId(string)
	SetBillingCycle(metalv1.DeviceCreateInputBillingCycle)
	SetSpotInstance(bool)
	SetSpotPriceMax(float32)
	SetTerminationTime(time.Time)
}

type OneOfDeviceCreator interface {
	DeviceCreator
	GetActualInstance() interface{}
}

var _ DeviceCreator = (*metal.DeviceCreateInMetroInput)(nil)
var _ DeviceCreator = (*metal.DeviceCreateInFacilityInput)(nil)

func (d *Driver) Create() error {
	var userdata string
	if d.UserDataFile != "" {
		buf, err := os.ReadFile(d.UserDataFile)
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

	d.SSHKeyID = key.GetId()

	hardwareReservationId := ""
	//check if hardware reservation requested
	if d.HardwareReserverationID != "" {
		hardwareReservationId = d.HardwareReserverationID
	}

	client := d.getClient()

	var dc DeviceCreator
	var createRequest metal.CreateDeviceRequest

	if d.Facility != "" {
		dc = &metal.DeviceCreateInFacilityInput{
			Facility: []string{d.Facility},
		}
		createRequest = metal.CreateDeviceRequest{DeviceCreateInFacilityInput: dc.(*metal.DeviceCreateInFacilityInput)}
	} else {
		dc = &metal.DeviceCreateInMetroInput{
			Metro: d.Metro,
		}
		createRequest = metal.CreateDeviceRequest{DeviceCreateInMetroInput: dc.(*metal.DeviceCreateInMetroInput)}
	}

	dc.SetHostname(d.MachineName)
	dc.SetPlan(d.Plan)
	dc.SetHardwareReservationId(hardwareReservationId)
	dc.SetOperatingSystem(d.OperatingSystem)
	dc.SetBillingCycle(metalv1.DeviceCreateInputBillingCycle(d.BillingCycle))
	dc.SetUserdata(userdata)
	dc.SetTags(d.Tags)
	dc.SetSpotInstance(d.SpotInstance)
	dc.SetSpotPriceMax(d.SpotPriceMax)
	if d.TerminationTime != nil {
		dc.SetTerminationTime(*d.TerminationTime)
	}

	log.Info("Provisioning Equinix Metal server...")
	newDevice, _, err := client.DevicesApi.CreateDevice(context.TODO(), d.ProjectID).CreateDeviceRequest(createRequest).Execute()
	if err != nil {
		log.Errorf("device could not be created: %s", err)

		//cleanup ssh keys if device failed
		if resp, err := client.SSHKeysApi.DeleteSSHKey(context.TODO(), d.SSHKeyID).Execute(); ignoreStatusCodes(resp, err, http.StatusForbidden, http.StatusNotFound) != nil {
			log.Errorf("ssh-key could not be deleted: %s", err)
			return err
		}
		return err
	}
	t0 := time.Now()

	d.DeviceID = newDevice.GetId()

	for {
		newDevice, _, err = client.DevicesApi.FindDeviceById(context.TODO(), d.DeviceID).Execute()
		if err != nil {
			return err
		}

		for _, ip := range newDevice.GetIpAddresses() {
			if ip.GetPublic() && ip.GetAddressFamily() == 4 {
				d.IPAddress = ip.GetAddress()
			}
		}

		if d.IPAddress != "" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	log.Infof("Created device ID %s, IP address %s",
		newDevice.GetId(),
		d.IPAddress)

	log.Info("Waiting for Provisioning...")
	stage := float32(0)
	for {
		newDevice, _, err = client.DevicesApi.FindDeviceById(context.TODO(), d.DeviceID).Execute()
		if err != nil {
			return err
		}
		if newDevice.GetState() == metal.DEVICESTATE_PROVISIONING && stage != newDevice.GetProvisioningPercentage() {
			stage = newDevice.GetProvisioningPercentage()
			log.Debugf("Provisioning %v%% complete", newDevice.GetProvisioningPercentage())
		}
		if newDevice.GetState() == metal.DEVICESTATE_ACTIVE {
			log.Debugf("Device State: %s", newDevice.GetState())
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

func (d *Driver) createSSHKey() (*metal.SSHKey, error) {
	sshKeyPath := d.GetSSHKeyPath()
	log.Debugf("Writing SSH Key to %s", sshKeyPath)

	if err := ssh.GenerateSSHKey(sshKeyPath); err != nil {
		return nil, err
	}

	publicKey, err := os.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		return nil, err
	}

	createRequest := metal.SSHKeyCreateInput{}
	createRequest.SetLabel(fmt.Sprintf("docker machine: %s", d.MachineName))
	createRequest.SetKey(string(publicKey))
	r := metal.ApiCreateSSHKeyRequest{}
	r.SSHKeyCreateInput(createRequest)

	key, _, err := d.getClient().SSHKeysApi.CreateSSHKeyExecute(r)
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
	device, _, err := d.getClient().DevicesApi.FindDeviceById(context.TODO(), d.DeviceID).Execute()
	if err != nil {
		return state.Error, err
	}

	switch device.GetState() {
	case metal.DEVICESTATE_QUEUED, metal.DEVICESTATE_PROVISIONING, metal.DEVICESTATE_POWERING_ON:
		return state.Starting, nil
	case metal.DEVICESTATE_ACTIVE:
		return state.Running, nil
	case metal.DEVICESTATE_POWERING_OFF:
		return state.Stopping, nil
	case metal.DEVICESTATE_INACTIVE:
		return state.Stopped, nil
	}
	return state.None, nil
}

func (d *Driver) Start() error {
	r := metal.DeviceActionInput{Type: metal.DEVICEACTIONINPUTTYPE_POWER_ON}
	_, err := d.getClient().DevicesApi.PerformAction(context.TODO(), d.DeviceID).DeviceActionInput(r).Execute()
	return err
}

func (d *Driver) Stop() error {
	r := metal.DeviceActionInput{Type: metal.DEVICEACTIONINPUTTYPE_POWER_OFF}
	_, err := d.getClient().DevicesApi.PerformAction(context.TODO(), d.DeviceID).DeviceActionInput(r).Execute()
	return err
}

func ignoreStatusCodes(resp *http.Response, err error, codes ...int) error {
	if err == nil && resp == nil {
		return nil
	}
	if err != nil {
		for _, c := range codes {
			if resp.StatusCode == c {
				return nil
			}
		}
	}

	return err
}

func (d *Driver) Remove() error {
	client := d.getClient()
	if resp, err := client.SSHKeysApi.DeleteSSHKey(context.TODO(), d.SSHKeyID).Execute(); ignoreStatusCodes(resp, err, http.StatusForbidden, http.StatusNotFound) != nil {
		return err
	}

	resp, err := client.DevicesApi.DeleteDevice(context.TODO(), d.DeviceID).Execute()
	return ignoreStatusCodes(resp, err, http.StatusForbidden, http.StatusNotFound)
}

func (d *Driver) Restart() error {
	r := metal.DeviceActionInput{Type: metal.DEVICEACTIONINPUTTYPE_REBOOT}
	_, err := d.getClient().DevicesApi.PerformAction(context.TODO(), d.DeviceID).DeviceActionInput(r).Execute()
	return err
}

func (d *Driver) Kill() error {
	return d.Stop()
}

func (d *Driver) GetDockerConfigDir() string {
	return dockerConfigDir
}

func (d *Driver) getClient() *metal.APIClient {
	config := metal.NewConfiguration()
	config.AddDefaultHeader("X-Consumer-Token", consumerToken)
	config.AddDefaultHeader("X-Auth-Token", d.ApiKey)
	userAgent := fmt.Sprintf("docker-machine-driver-%s/%s %s", d.DriverName(), version, config.UserAgent)
	if len(d.UserAgentPrefix) > 0 {
		userAgent = fmt.Sprintf("%s %s", d.UserAgentPrefix, userAgent)
	}
	config.UserAgent = userAgent
	client := metal.NewAPIClient(config)

	return client
}

func (d *Driver) getOsFlavors() ([]string, error) {
	operatingSystems, _, err := d.getClient().OperatingSystemsApi.FindOperatingSystems(context.TODO()).Execute()
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
	flavors := make([]string, 0, len(operatingSystems.OperatingSystems))
	for _, flavor := range operatingSystems.OperatingSystems {
		if stringInSlice(flavor.GetDistro(), supportedDistros) {
			flavors = append(flavors, flavor.GetSlug())
		}
	}
	return flavors, nil
}

func validateFacility(client *metal.APIClient, facility string) error {
	if facility == "any" {
		return nil
	}

	facilities, _, err := client.FacilitiesApi.FindFacilities(context.TODO()).Execute()
	if err != nil {
		return err
	}
	for _, f := range facilities.Facilities {
		if f.GetCode() == facility {
			return nil
		}
	}

	return fmt.Errorf("%s requires a valid facility", driverName)
}

func validateMetro(client *metal.APIClient, metro string) error {
	metros, _, err := client.MetrosApi.FindMetros(context.TODO()).Execute()
	if err != nil {
		return err
	}
	for _, m := range metros.Metros {
		if m.GetCode() == metro {
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
