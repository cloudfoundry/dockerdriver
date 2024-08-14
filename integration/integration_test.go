package integration_test

import (
	"context"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/driverhttp"
	"code.cloudfoundry.org/dockerdriver/integration"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Certify with: ", func() {
	var (
		err error

		testLogger           lager.Logger
		testContext          context.Context
		testEnv              dockerdriver.Env
		certificationFixture integration.CertificationFixture
		driverClient         dockerdriver.Driver
		errResponse          dockerdriver.ErrorResponse

		mountResponse dockerdriver.MountResponse
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("MainTest")
		testContext = context.TODO()
		testEnv = driverhttp.NewHttpDriverEnv(testLogger, testContext)

		fileName := os.Getenv("FIXTURE_FILENAME")
		Expect(fileName).NotTo(Equal(""))

		certificationFixture, err = integration.LoadCertificationFixture(fileName)
		Expect(err).NotTo(HaveOccurred())
		testLogger.Info("fixture", lager.Data{"filename": fileName, "context": certificationFixture})

		driverClient, err = driverhttp.NewRemoteClient(certificationFixture.DriverAddress, certificationFixture.TLSConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("given a driver", func() {
		It("should respond with Capabilities", func() {
			resp := driverClient.Capabilities(testEnv)
			Expect(resp.Capabilities).NotTo(BeNil())
			Expect(resp.Capabilities.Scope).To(Or(Equal("local"), Equal("global")))
		})
	})

	Context("given a created volume missing required options", func() {
		BeforeEach(func() {
			certificationFixture.CreateConfig.Name = "invalid-configuration"
			if _, found := certificationFixture.CreateConfig.Opts["password"]; !found {
				Skip("No password found in create config")
			}

			delete(certificationFixture.CreateConfig.Opts, "username")
			delete(certificationFixture.CreateConfig.Opts, "password")

			errResponse = driverClient.Create(testEnv, certificationFixture.CreateConfig)
			Expect(errResponse.Err).To(Equal(""))

		})

		AfterEach(func() {
			errResponse = driverClient.Unmount(testEnv, dockerdriver.UnmountRequest{
				Name: certificationFixture.CreateConfig.Name,
			})
			Expect(errResponse.Err).To(ContainSubstring("Volume invalid-configuration does not exist"))

			errResponse = driverClient.Remove(testEnv, dockerdriver.RemoveRequest{
				Name: certificationFixture.CreateConfig.Name,
			})
			Expect(errResponse.Err).To(ContainSubstring("Volume invalid-configuration does not exist"))
		})

		It("should log an error message", func() {
			mountResponse = driverClient.Mount(testEnv, dockerdriver.MountRequest{
				Name: certificationFixture.CreateConfig.Name,
			})

			Expect(mountResponse.Err).To(ContainSubstring("Missing mandatory options: username, password"))
		})
	})

	Context("given a created volume", func() {
		BeforeEach(func() {
			errResponse = driverClient.Create(testEnv, certificationFixture.CreateConfig)
			Expect(errResponse.Err).To(Equal(""))
		})

		AfterEach(func() {
			errResponse = driverClient.Remove(testEnv, dockerdriver.RemoveRequest{
				Name: certificationFixture.CreateConfig.Name,
			})
			Expect(errResponse.Err).To(Equal(""))
		})

		Context("given a mounted volume", func() {
			BeforeEach(func() {
				mountResponse = driverClient.Mount(testEnv, dockerdriver.MountRequest{
					Name: certificationFixture.CreateConfig.Name,
				})
				Expect(mountResponse.Err).To(Equal(""))
				Expect(mountResponse.Mountpoint).NotTo(Equal(""))
			})

			AfterEach(func() {
				errResponse = driverClient.Unmount(testEnv, dockerdriver.UnmountRequest{
					Name: certificationFixture.CreateConfig.Name,
				})
				Expect(errResponse.Err).To(Equal(""))
			})

			It("should mount that volume", func() {
				matches, err := filepath.Glob(mountResponse.Mountpoint)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(matches)).To(Equal(1))
			})

			It("should write to that volume", func() {
				testFileWrite(testLogger, mountResponse)
			})

			It("should not log any sensitive data", func() {
				createConfig := certificationFixture.CreateConfig
				if val, found := createConfig.Opts["password"]; found {
					driverOutput := string(session.Out.Contents())
					Expect(driverOutput).Should(ContainSubstring("REDACTED"))
					Expect(driverOutput).ShouldNot(ContainSubstring(val.(string)))
				} else {
					Skip("No password found in create config")
				}
			})
		})
	})

	It("should unmount a volume given same volume ID", func() {
		errResponse = driverClient.Create(testEnv, certificationFixture.CreateConfig)
		Expect(errResponse.Err).To(Equal(""))

		mountResponse := driverClient.Mount(testEnv, dockerdriver.MountRequest{
			Name: certificationFixture.CreateConfig.Name,
		})
		Expect(mountResponse.Err).To(Equal(""))

		errResponse = driverClient.Unmount(testEnv, dockerdriver.UnmountRequest{
			Name: certificationFixture.CreateConfig.Name,
		})
		Expect(errResponse.Err).To(Equal(""))
		Expect(cellClean(mountResponse.Mountpoint)).To(Equal(true))

		errResponse = driverClient.Remove(testEnv, dockerdriver.RemoveRequest{
			Name: certificationFixture.CreateConfig.Name,
		})
		Expect(errResponse.Err).To(Equal(""))

	})
})

// given a mounted mountpoint, tests creation of a file on that mount point
func testFileWrite(logger lager.Logger, mountResponse dockerdriver.MountResponse) {
	logger = logger.Session("test-file-write")
	logger.Info("start")
	defer logger.Info("end")

	fileName := "certtest-" + randomString(10)

	logger.Info("writing-test-file", lager.Data{"mountpoint": mountResponse.Mountpoint})
	testFile := path.Join(mountResponse.Mountpoint, fileName)
	logger.Info("writing-test-file", lager.Data{"filepath": testFile})
	err := os.WriteFile(testFile, []byte("hello persi"), 0644)
	Expect(err).NotTo(HaveOccurred())

	matches, err := filepath.Glob(mountResponse.Mountpoint + "/" + fileName)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(matches)).To(Equal(1))

	bytes, err := os.ReadFile(mountResponse.Mountpoint + "/" + fileName)
	Expect(err).NotTo(HaveOccurred())
	Expect(bytes).To(Equal([]byte("hello persi")))

	err = os.Remove(testFile)
	Expect(err).NotTo(HaveOccurred())

	matches, err = filepath.Glob(path.Join(mountResponse.Mountpoint, fileName))
	Expect(err).NotTo(HaveOccurred())
	Expect(len(matches)).To(Equal(0))
}

func cellClean(mountpoint string) bool {
	matches, err := filepath.Glob(mountpoint)
	Expect(err).NotTo(HaveOccurred())
	return len(matches) == 0
}

var isSeeded = false

func randomString(n int) string {
	if !isSeeded {
		rand.Seed(time.Now().UnixNano())
		isSeeded = true
	}
	runes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = runes[rand.Intn(len(runes))]
	}
	return string(b)
}
