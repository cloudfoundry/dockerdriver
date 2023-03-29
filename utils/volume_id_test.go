package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/dockerdriver/utils"
)

var _ = Describe("Utils/VolumeId", func() {
	var (
		volumeId utils.VolumeId
	)

	BeforeEach(func() {
		volumeId = utils.NewVolumeId("some_prefix", "some-suffix")
	})

	It("should return an appropriately formatted volumeid string", func() {
		Expect(volumeId.GetUniqueId()).To(Equal("some=prefix_some-suffix"))
	})

	It("should deserialize appropriately", func() {
		newVolumeId, err := utils.NewVolumeIdFromEncodedString("other-prefix_other=suffix")
		Expect(err).NotTo(HaveOccurred())

		Expect(newVolumeId).To(Equal(utils.VolumeId{Prefix: "other-prefix", Suffix: "other_suffix"}))
	})
})
