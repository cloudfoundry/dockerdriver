package integration

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/dockerdriver"
)

type Config struct {
	CreateConfig  dockerdriver.CreateRequest `json:"create_config"`
	Driver        string                     `json:"driver"`
	DriverName    string                     `json:"driver_name"`
	DriverAddress string                     `json:"driver_address"`
	DriverArgs    []string                   `json:"driver_args"`
	TLSConfig     *dockerdriver.TLSConfig    `json:"tls_config,omitempty"`
}

func LoadConfig() (Config, error) {
	fileName, avail := os.LookupEnv("CONFIG")
	if !avail {
		panic("CONFIG not set")
	}

	bytes, err := os.ReadFile(fileName)
	if err != nil {
		return Config{}, err
	}

	c := Config{}
	err = json.Unmarshal(bytes, &c)
	if err != nil {
		return Config{}, err
	}

	return c, nil
}
