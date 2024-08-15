package lazy_unmount_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/driverhttp"
	"code.cloudfoundry.org/dockerdriver/integration"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("LazyUnmount", func() {
	var (
		testLogger   lager.Logger
		testContext  context.Context
		testEnv      dockerdriver.Env
		config       integration.Config
		driverClient dockerdriver.Driver
		errResponse  dockerdriver.ErrorResponse

		mountResponse dockerdriver.MountResponse
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("LazyUnmountTest")
		testContext = context.TODO()
		testEnv = driverhttp.NewHttpDriverEnv(testLogger, testContext)

		var err error
		config, err = integration.LoadConfig()
		Expect(err).NotTo(HaveOccurred())
		testLogger.Info("fixture", lager.Data{"context": config})

		driverClient, err = driverhttp.NewRemoteClient(config.DriverAddress, config.TLSConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("given a created volume", func() {
		BeforeEach(func() {
			errResponse = driverClient.Create(testEnv, config.CreateConfig)
			Expect(errResponse.Err).To(Equal(""))
		})

		AfterEach(func() {
			errResponse = driverClient.Remove(testEnv, dockerdriver.RemoveRequest{
				Name: config.CreateConfig.Name,
			})
			Expect(errResponse.Err).To(Equal(""))
		})

		Context("given a mounted volume", func() {
			BeforeEach(func() {
				mountResponse = driverClient.Mount(testEnv, dockerdriver.MountRequest{
					Name: config.CreateConfig.Name,
				})
				Expect(mountResponse.Err).To(Equal(""))
				Expect(mountResponse.Mountpoint).NotTo(Equal(""))

				cmd := exec.Command("bash", "-c", "cat /proc/mounts | grep -E '"+mountResponse.Mountpoint+"'")
				Expect(cmdRunner(cmd)).To(Equal(0))
			})

			Context("when the nfs server has a file handle kept open during umount", func() {
				var file *os.File

				BeforeEach(func() {
					testFilePath := filepath.Join(mountResponse.Mountpoint, "file-used-to-keep-open")

					var err error
					file, err = os.OpenFile(testFilePath, os.O_CREATE, os.FileMode(0777))
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func() {
					_ = file.Close()
				})

				It("should unmount lazily", func() {
					errResponse := driverClient.Unmount(testEnv, dockerdriver.UnmountRequest{
						Name: config.CreateConfig.Name,
					})
					Expect(errResponse.Err).To(Equal(""))

					Eventually(func() int {
						cmd := exec.Command("bash", "-c", "cat /proc/mounts | grep -E '"+mountResponse.Mountpoint+"'")
						return cmdRunner(cmd)

					}, 5, 500*time.Millisecond).Should(Equal(1))
				})
			})
		})
	})
})

func cmdRunner(cmd *exec.Cmd) int {
	session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session, 10).Should(Exit())
	return session.ExitCode()
}
