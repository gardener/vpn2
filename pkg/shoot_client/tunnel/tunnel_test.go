package tunnel

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/network"
	"github.com/gardener/vpn2/pkg/vpn_client"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/vpn2/pkg/constants"
)

var _ = Describe("Tunnel controller helpers", func() {
	Describe("NewController", func() {
		It("initializes the controller map and nextClean", func() {
			c := NewController()
			Expect(c).NotTo(BeNil())
			Expect(c.kubeApiservers).NotTo(BeNil())
			Expect(c.nextClean.After(time.Now())).To(BeTrue())
			// nextClean should be roughly now + cleanUpPeriod
			Expect(c.nextClean.Before(time.Now().Add(cleanUpPeriod + time.Second))).To(BeTrue())
		})
	})

	Describe("kubeApiserverData.linkName", func() {
		It("builds a name using the last two bytes of the remote address", func() {
			// Build a 16-byte IPv6-like address and set last two bytes deterministically
			remote := net.ParseIP("2001:db8::1")
			remote[14] = 0xAB
			remote[15] = 0xCD

			d := &kubeApiserverData{
				remoteAddr: remote,
			}
			expected := fmt.Sprintf("%sip6tnl%02x%02x", constants.BondDevice, remote[len(remote)-2], remote[len(remote)-1])
			Expect(d.linkName()).To(Equal(expected))
		})
	})

	Describe("kubeApiserverData.needsUpdate", func() {
		var d *kubeApiserverData

		BeforeEach(func() {
			d = &kubeApiserverData{
				podIP: "10.0.0.1",
			}
		})

		It("returns true when pod IP changes", func() {
			Expect(d.needsUpdate("10.0.0.2")).To(BeTrue())
		})

		It("returns false when creation is complete", func() {
			d.creationComplete = true
			Expect(d.needsUpdate("10.0.0.1")).To(BeFalse())
		})

		It("respects creationFailureBackoff and returns false when lastCreationFailed is recent", func() {
			t := time.Now()
			d.lastCreationFailed = &t
			d.creationComplete = false
			Expect(d.needsUpdate("10.0.0.1")).To(BeFalse())
		})

		It("returns true when lastCreationFailed is older than backoff", func() {
			old := time.Now().Add(-creationFailureBackoff - time.Second)
			d.lastCreationFailed = &old
			d.creationComplete = false
			Expect(d.needsUpdate("10.0.0.1")).To(BeTrue())
		})
	})

	Describe("kubeApiserverData.isOutdated", func() {
		It("returns true when lastSeen is older than expirationDuration", func() {
			d := &kubeApiserverData{}
			d.lastSeen = time.Now().Add(-expirationDuration - time.Second)
			Expect(d.isOutdated()).To(BeTrue())
		})

		It("returns false when lastSeen is recent", func() {
			d := &kubeApiserverData{}
			d.lastSeen = time.Now()
			Expect(d.isOutdated()).To(BeFalse())
		})
	})
})

var _ = Describe("Controller Run", func() {
	var (
		log logr.Logger
		cfg *config.VPNClient
	)

	BeforeEach(func() {
		log, _ = logger.NewZapLogger("debug", "text", zap.StacktraceLevel(zapcore.PanicLevel))
		ctx := context.Background()
		cfg = &config.VPNClient{
			IsShootClient:  true,
			HAVPNServers:   uint(2),
			VPNNetwork:     network.ParseIPNetIgnoreError(constants.DefaultVPNNetwork.String()),
			VPNClientIndex: 0,
		}
		err := exec.Command("mkdir", "-p", "/dev/net").Run()
		Expect(err).NotTo(HaveOccurred())
		err = exec.Command("mknod", "/dev/net/tun", "c", "10", "200").Run()
		Expect(err).NotTo(HaveOccurred())
		err = vpn_client.ConfigureBonding(ctx, log, cfg)
		Expect(err).NotTo(HaveOccurred())
	})
	AfterEach(func() {
		// clean up created tap devices
		err := network.DeleteLinkByName("bond0")
		Expect(err).NotTo(HaveOccurred())
		for i := 0; i < 2; i++ {
			linkName := fmt.Sprintf("tap%d", i)
			err := network.DeleteLinkByName(linkName)
			Expect(err).NotTo(HaveOccurred())
		}
	})
	It("listens for UDP6 and registers kube-apiserver", func() {

		c := NewController()

		// run controller in background
		go func() {
			// Run may block; we ignore returned error for the test run
			_ = c.Run(log)
		}()

		// give the server a moment to start listening
		time.Sleep(200 * time.Millisecond)

		// send a single UDP6 packet to the controller
		ip := network.BondingShootClientAddress(cfg.VPNNetwork.ToIPNet(), cfg.VPNClientIndex)
		addr := &net.UDPAddr{IP: ip.IP, Port: tunnelControllerPort}
		conn, err := net.DialUDP("udp6", nil, addr)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()

		_, err = conn.Write([]byte("10.0.0.5"))
		Expect(err).ToNot(HaveOccurred())

		// eventually the controller should have an entry for this kube-apiserver
		Eventually(func() int {
			c.lock.Lock()
			n := len(c.kubeApiservers)
			c.lock.Unlock()
			return n
		}, "3s", "100ms").Should(BeNumerically(">=", 1))
	})
})
