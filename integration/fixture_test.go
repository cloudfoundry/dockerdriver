package integration_test

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("certification/fixture.go", func() {
	var (
		err                  error
		tmpDir, tmpFileName  string
		certificationFixture integration.CertificationFixture
	)

	BeforeEach(func() {
		tmpDir, err = os.MkdirTemp("", "certification")
		Expect(err).NotTo(HaveOccurred())

		tmpFile, err := os.CreateTemp(tmpDir, "certification-fixture.json")
		Expect(err).NotTo(HaveOccurred())

		tmpFileName = tmpFile.Name()
		tmpFile.Close()

		certificationFixture = integration.CertificationFixture{}
	})

	AfterEach(func() {
		err = os.RemoveAll(tmpDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("#LoadCertificationFixture", func() {
		BeforeEach(func() {
			certificationFixtureContent := `{
 						"volman_driver_path": "fake-path-to-driver",
  					"driver_address": "http://fakedriver_address",
  					"driver_name": "fakedriver",
						"create_config": {
						    "Name": "fake-request",
						    "Opts": {"key":"value"}
 						},
						"tls_config": {
								"InsecureSkipVerify": true,
								"CAFile": "fakedriver_ca.crt",
								"CertFile":"fakedriver_client.crt",
								"KeyFile":"fakedriver_client.key"
							}
						}`

			err = os.WriteFile(tmpFileName, []byte(certificationFixtureContent), 0666)
			Expect(err).NotTo(HaveOccurred())
		})

		It("loads the fake certification fixture", func() {
			certificationFixture, err = integration.LoadCertificationFixture(tmpFileName)
			Expect(err).NotTo(HaveOccurred())

			Expect(certificationFixture.VolmanDriverPath).To(ContainSubstring("fake-path-to-driver"))
			Expect(certificationFixture.CreateConfig.Name).To(Equal("fake-request"))
		})
	})

	Context("#SaveCertificationFixture", func() {
		BeforeEach(func() {
			certificationFixture = integration.CertificationFixture{
				VolmanDriverPath: "fake-path-to-driver",
				DriverName:       "fakedriver",
				CreateConfig: dockerdriver.CreateRequest{
					Name: "fake-request",
					Opts: map[string]interface{}{"key": "value"},
				},
			}
		})

		It("saves the fake certification fixture", func() {
			err = integration.SaveCertificationFixture(certificationFixture, tmpFileName)
			Expect(err).NotTo(HaveOccurred())

			bytes, err := os.ReadFile(tmpFileName)
			Expect(err).ToNot(HaveOccurred())

			readFixture := integration.CertificationFixture{}
			json.Unmarshal(bytes, &readFixture)

			Expect(readFixture.VolmanDriverPath).To(Equal("fake-path-to-driver"))
			Expect(readFixture.CreateConfig.Name).To(Equal("fake-request"))
		})
	})

})
