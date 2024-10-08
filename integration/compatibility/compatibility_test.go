package compatibility_test

import (
	"context"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/driverhttp"
	"code.cloudfoundry.org/dockerdriver/integration"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Compatibility", func() {
	var (
		testLogger   lager.Logger
		testContext  context.Context
		testEnv      dockerdriver.Env
		driverClient dockerdriver.Driver
		errResponse  dockerdriver.ErrorResponse
		config       integration.Config

		mountResponse dockerdriver.MountResponse
	)

	BeforeEach(func() {
		testLogger = lagertest.NewTestLogger("CompatibilityTest")
		testContext = context.TODO()
		testEnv = driverhttp.NewHttpDriverEnv(testLogger, testContext)

		var err error
		config, err = integration.LoadConfig()
		Expect(err).NotTo(HaveOccurred())

		driverClient, err = driverhttp.NewRemoteClient(config.DriverAddress, config.TLSConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	It("verifies driver name", func() {
		Expect([]string{"smbdriver", "nfsv3driver"}).To(ContainElement(config.DriverName))
	})

	DescribeTable("nfs",
		func(values map[string]interface{}) {
			if config.DriverName != "nfsv3driver" {
				Skip("This is for nfsv3driver only")
				return
			}
			cf := config
			cf.CreateConfig = dockerdriver.CreateRequest{}
			cf.CreateConfig.Name = uuid.NewString()
			cf.CreateConfig.Opts = map[string]interface{}{}
			cf.CreateConfig.Opts["source"] = config.CreateConfig.Opts["source"]
			cf.CreateConfig.Opts["uid"] = config.CreateConfig.Opts["uid"]
			cf.CreateConfig.Opts["gid"] = config.CreateConfig.Opts["gid"]
			cf.CreateConfig.Opts["auto_cache"] = "true"
			for index, value := range values {
				cf.CreateConfig.Opts[index] = value
			}
			testLogger.Info("using fixture", lager.Data{"fixture": cf})
			errResponse = driverClient.Create(testEnv, cf.CreateConfig)
			Expect(errResponse.Err).To(Equal(""))

			mountResponse = driverClient.Mount(testEnv, dockerdriver.MountRequest{
				Name: cf.CreateConfig.Name,
			})
			Expect(mountResponse.Err).To(Equal(""))
			Expect(mountResponse.Mountpoint).NotTo(Equal(""))

			cmd := exec.Command("bash", "-c", "cat /proc/mounts | grep -E '"+mountResponse.Mountpoint+"'")
			Expect(cmdRunner(cmd)).To(Equal(0))
			if isReadWrite(cf.CreateConfig.Opts) {
				testFileWrite(testLogger, mountResponse)
			} else {
				testReadOnly(testLogger, mountResponse)
			}

			// Cleanup
			errResponse = driverClient.Unmount(testEnv, dockerdriver.UnmountRequest{
				Name: cf.CreateConfig.Name,
			})
			Expect(errResponse.Err).To(Equal(""))

			errResponse = driverClient.Remove(testEnv, dockerdriver.RemoveRequest{
				Name: cf.CreateConfig.Name,
			})
			Expect(errResponse.Err).To(Equal(""))
		},
		Entry("default", map[string]interface{}{}),
		Entry("when experimental=true", map[string]interface{}{"experimental": "true"}),
		Entry("when mount=/foo/bar", map[string]interface{}{"mount": "/foo/bar"}),
		Entry("when experimental=true and version=4", map[string]interface{}{"experimental": "true", "version": "4"}),
		Entry("when version=4.1", map[string]interface{}{"version": "4.1"}),
		Entry("when mount=/foo/bar and cache=true", map[string]interface{}{"mount": "/foo/bar", "cache": "true"}),
	)
	DescribeTable("smb",
		func(key string, value interface{}) {
			if config.DriverName != "smbdriver" {
				Skip("This is for smbdriver only")
				return
			}
			cf := config
			cf.CreateConfig = dockerdriver.CreateRequest{}
			cf.CreateConfig.Name = uuid.NewString()
			cf.CreateConfig.Opts = map[string]interface{}{}
			cf.CreateConfig.Opts["source"] = config.CreateConfig.Opts["source"]
			cf.CreateConfig.Opts["username"] = config.CreateConfig.Opts["username"]
			cf.CreateConfig.Opts["password"] = config.CreateConfig.Opts["password"]
			cf.CreateConfig.Opts[key] = value

			testLogger.Info("using fixture", lager.Data{"fixture": cf})
			errResponse = driverClient.Create(testEnv, cf.CreateConfig)
			Expect(errResponse.Err).To(Equal(""))

			mountResponse = driverClient.Mount(testEnv, dockerdriver.MountRequest{
				Name: cf.CreateConfig.Name,
			})
			Expect(mountResponse.Err).To(Equal(""))
			Expect(mountResponse.Mountpoint).NotTo(Equal(""))

			cmd := exec.Command("bash", "-c", "cat /proc/mounts | grep -E '"+mountResponse.Mountpoint+"'")
			Expect(cmdRunner(cmd)).To(Equal(0))
			if isReadWrite(cf.CreateConfig.Opts) {
				testFileWrite(testLogger, mountResponse)
			} else {
				testReadOnly(testLogger, mountResponse)
			}

			// Cleanup
			errResponse = driverClient.Unmount(testEnv, dockerdriver.UnmountRequest{
				Name: cf.CreateConfig.Name,
			})
			Expect(errResponse.Err).To(Equal(""))

			errResponse = driverClient.Remove(testEnv, dockerdriver.RemoveRequest{
				Name: cf.CreateConfig.Name,
			})
			Expect(errResponse.Err).To(Equal(""))
		},
		Entry("with a default volume mount", "domain", ""),
		Entry("with a readonly=true volume mount", "readonly", true),
		Entry("with a ro=true volume mount", "ro", "true"),
		Entry("with a mount=/foo/bar volume mount", "mount", "/foo/bar"),
		Entry("with a version=1.0 volume mount", "version", "1.0"),
		Entry("with a version=2.0 volume mount", "version", "2.0"),
		Entry("with a version=2.1 volume mount", "version", "2.1"),
		Entry("with a version=3.0 volume mount", "version", "3.0"),
		Entry("with a version=3.1.1 volume mount", "version", "3.1.1"),
		Entry("with a mfsymlinks=true volume mount", "mfsymlinks", "true"),
	)
})

func testFileWrite(logger lager.Logger, mountResponse dockerdriver.MountResponse) {
	logger = logger.Session("test-file-write")
	logger.Info("start")
	defer logger.Info("end")

	fileName := "certtest-" + uuid.NewString()

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

func isReadWrite(opts map[string]interface{}) bool {
	if _, ok := opts["readonly"]; ok {
		return false
	} else if _, ok := opts["ro"]; ok {
		return false
	} else {
		return true
	}
}

func testReadOnly(logger lager.Logger, mountResponse dockerdriver.MountResponse) {
	logger = logger.Session("test-read-only")
	logger.Info("start")
	defer logger.Info("end")

	fileName := "certtest-" + uuid.NewString()

	logger.Info("writing-test-file", lager.Data{"mountpoint": mountResponse.Mountpoint})
	testFile := path.Join(mountResponse.Mountpoint, fileName)
	logger.Info("writing-test-file", lager.Data{"filepath": testFile})
	err := os.WriteFile(testFile, []byte("hello persi"), 0644)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("read-only file system"))
}

func cmdRunner(cmd *exec.Cmd) int {
	session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session, 10).Should(Exit())
	return session.ExitCode()
}
