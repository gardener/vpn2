package health

import (
	"io"
	"os"
	"regexp"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OpenVPN Server Status", func() {
	var status *OpenVPNStatus
	var err error

	Context("empty server", func() {
		BeforeEach(func() {
			status, err = ParseFile("test/openvpn-empty.status")
			Expect(status).ToNot(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should report vpn-server as down if timestamp is outdated", func() {
			status.UpdatedAt = time.Now().Add(-20 * time.Second)
			Expect(isUp(status, 15)).To(BeFalse())
		})
		It("should report vpn-server as up if timestamp is not outdated", func() {
			status.UpdatedAt = time.Now().Add(-2 * time.Second)
			Expect(isUp(status, 15)).To(BeTrue())
		})
		It("should have zero clients", func() {
			Expect(len(status.Clients)).To(Equal(0))
		})
		It("should have zero routing entries", func() {
			Expect(len(status.RoutingTable)).To(Equal(0))
		})
		It("should be ready in non-HA mode", func() {
			Expect(isReady(status, false)).To(BeTrue())
		})
		It("should not be ready in HA mode", func() {
			Expect(isReady(status, true)).To(BeFalse())
		})
	})

	Context("server with only seed clients", func() {
		BeforeEach(func() {
			status, err = ParseFile(`test/openvpn-not-ready.status`)
			Expect(status).ToNot(BeNil())
			Expect(err).NotTo(HaveOccurred())
			status.UpdatedAt = time.Now().Add(-2 * time.Second)
		})

		It("should report vpn-server as up", func() {
			Expect(isUp(status, 15)).To(BeTrue())
		})
		It("should have three clients", func() {
			Expect(len(status.Clients)).To(Equal(3))
		})
		It("should have routing entries", func() {
			Expect(len(status.RoutingTable)).To(BeNumerically(">", 0))
		})
		It("should not be ready (non-HA)", func() {
			Expect(isReady(status, false)).To(BeFalse())
		})
		It("should not be ready (HA)", func() {
			Expect(isReady(status, true)).To(BeFalse())
		})
	})

	Context("server with both seed and shoot clients", func() {
		BeforeEach(func() {
			status, err = ParseFile(`test/openvpn-ready.status`)
			Expect(status).ToNot(BeNil())
			Expect(err).NotTo(HaveOccurred())
			status.UpdatedAt = time.Now().Add(-2 * time.Second)
		})

		It("should report vpn-server as up", func() {
			Expect(isUp(status, 15)).To(BeTrue())
		})
		It("should have five clients", func() {
			Expect(len(status.Clients)).To(Equal(5))
		})
		It("should have routing entries", func() {
			Expect(len(status.RoutingTable)).To(BeNumerically(">", 0))
		})
		It("should be ready (non-HA)", func() {
			Expect(isReady(status, false)).To(BeTrue())
		})
		It("should be ready (HA)", func() {
			Expect(isReady(status, true)).To(BeTrue())
		})
	})

	Context("server with only shoot clients", func() {
		BeforeEach(func() {
			status, err = ParseFile(`test/openvpn-nonha-ready.status`)
			Expect(status).ToNot(BeNil())
			Expect(err).NotTo(HaveOccurred())
			status.UpdatedAt = time.Now().Add(-2 * time.Second)
		})

		It("should report vpn-server as up", func() {
			Expect(isUp(status, 15)).To(BeTrue())
		})
		It("should have one client", func() {
			Expect(len(status.Clients)).To(Equal(1))
		})
		It("should have routing entries", func() {
			Expect(len(status.RoutingTable)).To(BeNumerically(">", 0))
		})
		It("should be ready (non-HA)", func() {
			Expect(isReady(status, false)).To(BeTrue())
		})
		It("should not be ready (HA)", func() {
			Expect(isReady(status, true)).To(BeFalse())
		})
	})

	Context("server with bad status file", func() {
		var badStatusFile *os.File

		// Helper function that updates a file in-place based on regex
		sed := func(filePath string, pattern string, replacement string) error {
			content, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}

			re := regexp.MustCompile("(?m)" + pattern)
			updatedContent := re.ReplaceAllString(string(content), replacement)

			return os.WriteFile(filePath, []byte(updatedContent), 0644)
		}

		BeforeEach(func() {
			badStatusFile, err = os.CreateTemp("", "openvpn-bad-*.status")
			Expect(err).NotTo(HaveOccurred())
			goodStatusFile, err := os.Open("test/openvpn-ready.status")
			Expect(err).NotTo(HaveOccurred())
			_, err = io.Copy(badStatusFile, goodStatusFile)
			Expect(err).NotTo(HaveOccurred())
			_ = goodStatusFile.Close()
			_ = badStatusFile.Close()
		})
		AfterEach(func() {
			err := os.Remove(badStatusFile.Name())
			Expect(err).NotTo(HaveOccurred())
		})
		It("should report a bad TITLE line", func() {
			err := sed(badStatusFile.Name(), `^TITLE`, `BAD_TITLE`)
			Expect(err).NotTo(HaveOccurred())

			status, err = ParseFile(badStatusFile.Name())
			Expect(status).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown line type: BAD_TITLE"))
		})
		It("should report a bad TIME line", func() {
			err := sed(badStatusFile.Name(), `^TIME,\d{4}-\d{2}-\d{2}\W\d{2}:\d{2}:\d{2},\d*`, `TIME,BAD_TIMESTAMP`)
			Expect(err).NotTo(HaveOccurred())

			status, err = ParseFile(badStatusFile.Name())
			Expect(status).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot parse \"BAD_TIMESTAMP\""))
		})
		It("should ignore a bad HEADER line", func() {
			err := sed(badStatusFile.Name(), `^HEADER`, `HEADER,BAD,LINE`)
			Expect(err).NotTo(HaveOccurred())

			status, err = ParseFile(badStatusFile.Name())
			Expect(status).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})
		It("should report a bad CLIENT_LIST line", func() {
			err := sed(badStatusFile.Name(), `^CLIENT_LIST,vpn-seed-client,100.64.3.32:41392,.*`, `CLIENT_LIST,100.64.3.32:41392,,fd8f:6d53:b97a:1::100:2,10946525,21923133,2025-12-19 07:13:24,1766128404,UNDEF,30,0,AES-256-GCM`)
			Expect(err).NotTo(HaveOccurred())

			status, err = ParseFile(badStatusFile.Name())
			Expect(status).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid CLIENT_LIST line"))
		})
		It("should report a bad ROUTING_TABLE line", func() {
			err := sed(badStatusFile.Name(), `^ROUTING_TABLE,de:23:94:06:67:04@0,vpn-seed-client,.*`, `ROUTING_TABLE,100.64.3.32:41392,2025-12-19 11:12:03,1766142723`)
			Expect(err).NotTo(HaveOccurred())

			status, err = ParseFile(badStatusFile.Name())
			Expect(status).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid ROUTING_TABLE line"))
		})
		It("should report a bad GLOBAL_STATS line", func() {
			err := sed(badStatusFile.Name(), `^GLOBAL_STATS,Max bcast/mcast queue length,16`, `GLOBAL_STATS`)
			Expect(err).NotTo(HaveOccurred())

			status, err = ParseFile(badStatusFile.Name())
			Expect(status).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid GLOBAL_STATS line"))
		})
		It("should report unknown status lines", func() {
			err := sed(badStatusFile.Name(), `^END`, `END_OF_FILE`)
			Expect(err).NotTo(HaveOccurred())

			status, err = ParseFile(badStatusFile.Name())
			Expect(status).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown line type: END_OF_FILE"))
		})
	})
})
