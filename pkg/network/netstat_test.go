package network

import (
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetNetStats", func() {
	var (
		fixtureRoot string
	)

	BeforeEach(func() {
		fixtureRoot = filepath.Join("test")
	})

	Context("good scenario", func() {
		It("aggregates stats without packet loss", func() {
			stats, err := GetNetStats(filepath.Join(fixtureRoot, "good"))
			Expect(err).ToNot(HaveOccurred())

			Expect(stats).To(HaveKey("Tcp"))
			Expect(stats["Tcp"]).To(HaveKey("OutSegs"))
			Expect(stats["Tcp"]).To(HaveKey("RetransSegs"))

			// Values based on provided fixtures
			Expect(stats["Tcp"]["OutSegs"]).To(Equal("777437"))
			Expect(stats["Tcp"]["RetransSegs"]).To(Equal("1"))
		})
	})

	Context("ipv4 single stack scenario", func() {
		It("aggregates stats without snmp6", func() {
			stats, err := GetNetStats(filepath.Join(fixtureRoot, "ipv4only"))
			Expect(err).ToNot(HaveOccurred())

			Expect(stats).To(HaveKey("Tcp"))
			Expect(stats["Tcp"]).To(HaveKey("OutSegs"))
			Expect(stats["Tcp"]).To(HaveKey("RetransSegs"))

			// Values based on provided fixtures
			Expect(stats["Tcp"]["OutSegs"]).To(Equal("777437"))
			Expect(stats["Tcp"]["RetransSegs"]).To(Equal("1"))
		})
	})

	Context("packet loss scenario", func() {
		It("aggregates stats and shows packet loss", func() {
			stats, err := GetNetStats(filepath.Join(fixtureRoot, "packetloss"))
			Expect(err).ToNot(HaveOccurred())

			Expect(stats).To(HaveKey("Tcp"))
			Expect(stats["Tcp"]).To(HaveKey("OutSegs"))
			Expect(stats["Tcp"]).To(HaveKey("RetransSegs"))

			// Values based on provided fixtures
			Expect(stats["Tcp"]["OutSegs"]).To(Equal("835795"))
			Expect(stats["Tcp"]["RetransSegs"]).To(Equal("208948"))

			// Expect about 25% packet loss
			outSegs, err := strconv.ParseFloat(stats["Tcp"]["OutSegs"], 64)
			Expect(err).ToNot(HaveOccurred())
			retransSegs, err := strconv.ParseFloat(stats["Tcp"]["RetransSegs"], 64)
			Expect(err).ToNot(HaveOccurred())
			lossRatio := retransSegs / outSegs
			Expect(lossRatio).To(BeNumerically("~", 0.25, 0.01))
		})
	})

	Context("missing dir and net files", func() {
		It("returns an error if required files are missing", func() {
			_, err := GetNetStats(filepath.Join(fixtureRoot, "does-not-exist"))
			Expect(err).To(HaveOccurred())
		})
	})

	Context("correct dir but missing files", func() {
		It("returns an error if required files are missing under the correct path", func() {
			_, err := GetNetStats(filepath.Join(fixtureRoot, "missing"))
			Expect(err).To(HaveOccurred())
		})
	})

	Context("corrupt files", func() {
		It("returns an error if files under /proc/net/ are corrupt (netstat)", func() {
			_, err := GetNetStats(filepath.Join(fixtureRoot, "corrupt"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("mismatch field count mismatch"))
		})
		It("returns an error if files under /proc/net/ are corrupt (snmp)", func() {
			_, err := GetNetStats(filepath.Join(fixtureRoot, "corrupt2"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("header to value mismatch in snmp: Ip: vs Icmp:"))
		})
		It("drops corrupt icmp6 metrics", func() {
			stats, err := GetNetStats(filepath.Join(fixtureRoot, "corrupt3"))
			Expect(err).NotTo(HaveOccurred())
			Expect(stats["Icmp6"]).ToNot(HaveKey("BadMetrics"))
			Expect(stats["Icmp6"]).ToNot(HaveKey("Ip6MultipleValues"))
		})
	})

})
