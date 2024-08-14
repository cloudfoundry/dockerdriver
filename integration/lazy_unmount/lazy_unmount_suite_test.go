package lazy_unmount_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

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
	cmd := exec.Command(os.Getenv("DRIVER_CMD"), strings.Split(os.Getenv("DRIVER_OPTS"), ",")...)

	var err error
	session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session.Out).Should(gbytes.Say("driver-server.started"))
})
