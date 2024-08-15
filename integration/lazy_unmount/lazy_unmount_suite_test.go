package lazy_unmount_test

import (
	"os/exec"
	"testing"

	"code.cloudfoundry.org/dockerdriver/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var session *gexec.Session

func TestLazyUnmount(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LazyUnmount Suite")
}

var _ = BeforeSuite(func() {
	config, err := integration.LoadConfig()
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command(config.Driver, config.DriverArgs...)

	session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session.Out).Should(gbytes.Say("driver-server.started"))
})
var _ = AfterSuite(func() {
	session.Interrupt()
	Eventually(session).Should(gexec.Exit())
})
