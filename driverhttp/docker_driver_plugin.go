package driverhttp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/volman"
)

type DockerDriverPlugin struct {
	Driver interface{}
}

func NewDockerPluginWithDriver(driver voldriver.Driver) volman.Plugin {
	return &DockerDriverPlugin{
		Driver: driver,
	}
}

func (dw *DockerDriverPlugin) Matches(logger lager.Logger, pluginSpec volman.PluginSpec) bool {
	logger = logger.Session("matches")
	logger.Info("start")
	defer logger.Info("end")

	var matches bool
	matchableDriver, ok := dw.Driver.(voldriver.MatchableDriver)
	logger.Info("matches", lager.Data{"is-matchable": ok})
	if ok {
		var tlsConfig *voldriver.TLSConfig
		if pluginSpec.TLSConfig != nil {
			tlsConfig = &voldriver.TLSConfig{
				InsecureSkipVerify: pluginSpec.TLSConfig.InsecureSkipVerify,
				CAFile:             pluginSpec.TLSConfig.CAFile,
				CertFile:           pluginSpec.TLSConfig.CertFile,
				KeyFile:            pluginSpec.TLSConfig.KeyFile,
			}
		}
		matches = matchableDriver.Matches(logger, pluginSpec.Address, tlsConfig)
	}
	logger.Info("matches", lager.Data{"matches": matches})
	return matches
}

func (d *DockerDriverPlugin) Mount(logger lager.Logger, driverId string, volumeId string, opts map[string]interface{}) (volman.MountResponse, error) {
	env := NewHttpDriverEnv(logger, context.TODO())

	logger.Debug("creating-volume", lager.Data{"volumeId": volumeId, "driverId": driverId})
	response := d.Driver.(voldriver.Driver).Create(env, voldriver.CreateRequest{Name: volumeId, Opts: opts})
	if response.Err != "" {
		return volman.MountResponse{}, errors.New(response.Err)
	}

	mountRequest := voldriver.MountRequest{Name: volumeId}
	logger.Debug("calling-driver-with-mount-request", lager.Data{"driverId": driverId, "mountRequest": mountRequest})
	mountResponse := d.Driver.(voldriver.Driver).Mount(env, mountRequest)
	logger.Debug("response-from-driver", lager.Data{"response": mountResponse})

	if !strings.HasPrefix(mountResponse.Mountpoint, "/var/vcap/data") {
		logger.Info("invalid-mountpath", lager.Data{"detail": fmt.Sprintf("Invalid or dangerous mountpath %s outside of /var/vcap/data", mountResponse.Mountpoint)})
	}

	if mountResponse.Err != "" {
		return volman.MountResponse{}, errors.New(mountResponse.Err)
	}

	return volman.MountResponse{mountResponse.Mountpoint}, nil
}

func (d *DockerDriverPlugin) GetImplementation() interface{} {
	return d.Driver
}
