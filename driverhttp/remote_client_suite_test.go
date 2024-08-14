package driverhttp_test

import (
	"fmt"
	"io"
	"os/exec"
	"path"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var localDriverPath string

func TestDriver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Docker Driver Remote Client and Handlers Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	dirname := GinkgoT().TempDir()
	cmd := exec.Command("go", "install", "-race", "code.cloudfoundry.org/localdriver/cmd/localdriver@latest")
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("GOBIN=%s", dirname))
	cmd.Dir = dirname

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session).WithTimeout(time.Hour).Should(gexec.Exit(0))

	localDriverPath = path.Join(dirname, "localdriver")
	Expect(localDriverPath).To(BeAnExistingFile())
	return []byte(localDriverPath)
}, func(pathsByte []byte) {
	localDriverPath = string(pathsByte)
})

// testing support types:

type errCloser struct{ io.Reader }

func (errCloser) Close() error                     { return nil }
func (errCloser) Read(p []byte) (n int, err error) { return 0, fmt.Errorf("any") }

type stringCloser struct{ io.Reader }

func (stringCloser) Close() error { return nil }
