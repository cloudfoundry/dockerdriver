package driverhttp_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/voldriver/voldriverfakes"

	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/volman"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("DockerDriverMounter", func() {
	var (
		volumeId      string
		logger        *lagertest.TestLogger
		dockerPlugin  volman.Plugin
		fakeVoldriver *voldriverfakes.FakeDriver
	)

	BeforeEach(func() {
		volumeId = "fake-volume"
		logger = lagertest.NewTestLogger("docker-mounter-test")
		fakeVoldriver = &voldriverfakes.FakeDriver{}
		dockerPlugin = driverhttp.NewDockerPluginWithDriver(fakeVoldriver)
	})

	Describe("Mount", func() {
		Context("when given a driver", func() {

			Context("mount", func() {

				BeforeEach(func() {
					mountResponse := voldriver.MountResponse{Mountpoint: "/var/vcap/data/mounts/" + volumeId}
					fakeVoldriver.MountReturns(mountResponse)
				})

				It("should be able to mount without warning", func() {
					mountPath, err := dockerPlugin.Mount(logger, volumeId, map[string]interface{}{"volume_id": volumeId})
					Expect(err).NotTo(HaveOccurred())
					Expect(mountPath).NotTo(Equal(""))
					Expect(logger.Buffer()).NotTo(gbytes.Say("Invalid or dangerous mountpath"))
				})

				It("should not be able to mount if mount fails", func() {
					mountResponse := voldriver.MountResponse{Err: "an error"}
					fakeVoldriver.MountReturns(mountResponse)
					_, err := dockerPlugin.Mount(logger, volumeId, map[string]interface{}{"volume_id": volumeId})
					Expect(err).To(HaveOccurred())
				})

				Context("with bad mount path", func() {
					var err error
					BeforeEach(func() {
						mountResponse := voldriver.MountResponse{Mountpoint: "/var/tmp"}
						fakeVoldriver.MountReturns(mountResponse)
					})

					JustBeforeEach(func() {
						_, err = dockerPlugin.Mount(logger, volumeId, map[string]interface{}{"volume_id": volumeId})
					})

					It("should return a warning in the log", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(logger.Buffer()).To(gbytes.Say("Invalid or dangerous mountpath"))
					})
				})

			})
		})
	})

	Describe("Unmount", func() {
		It("should be able to unmount", func() {
			err := dockerPlugin.Unmount(logger, volumeId)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeVoldriver.UnmountCallCount()).To(Equal(1))
			Expect(fakeVoldriver.RemoveCallCount()).To(Equal(0))
		})

		It("should not be able to unmount when driver unmount fails", func() {
			fakeVoldriver.UnmountReturns(voldriver.ErrorResponse{Err: "unmount failure"})
			err := dockerPlugin.Unmount(logger, volumeId)
			Expect(err).To(HaveOccurred())
		})
	})
})
