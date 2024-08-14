package compatibility_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"code.cloudfoundry.org/dockerdriver/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

type VolumeServiceBrokerBinding struct {
	VolumeMounts []struct {
		Device struct {
			VolumeID    string                 `json:"volume_id"`
			MountConfig map[string]interface{} `json:"mount_config"`
		} `json:"device"`
	} `json:"volume_mounts"`
}

var (
	integrationFixtureTemplate = integration.LoadFixtureTemplate()
	bindingsFixture            = LoadVolumeServiceBrokerBindingsFixture()
	session                    *gexec.Session
)

func TestCompatibility(t *testing.T) {

	RegisterFailHandler(Fail)
	RunSpecs(t, "Compatibility Suite")
}

var _ = BeforeSuite(func() {
	cmd := exec.Command(os.Getenv("DRIVER_CMD"), strings.Split(os.Getenv("DRIVER_OPTS"), ",")...)

	var err error
	session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Eventually(session.Out).Should(gbytes.Say("driver-server.started"))
})

func LoadVolumeServiceBrokerBindingsFixture() []VolumeServiceBrokerBinding {
	var ok bool
	var bindingsFile string
	if bindingsFile, ok = os.LookupEnv("BINDINGS_FILE"); !ok {
		panic(errors.New("BINDINGS_FILE environment variable not set"))
	}

	bytes, err := os.ReadFile(bindingsFile)
	if err != nil {
		panic(err.Error())
	}

	bindings := []VolumeServiceBrokerBinding{}
	err = json.Unmarshal(bytes, &bindings)
	if err != nil {
		panic(err.Error())
	}

	return bindings
}
