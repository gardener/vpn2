package vpn_client

import (
	"context"
	"os/exec"
	"testing"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/vpn2/pkg/config"
	"github.com/gardener/vpn2/pkg/constants"
	"github.com/gardener/vpn2/pkg/network"
)

func TestBonding(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bonding Suite")
}

var _ = Describe("ConfigureBonding", Serial, func() {
	var (
		ctx context.Context
		log logr.Logger
		cfg *config.VPNClient
	)

	BeforeEach(func() {
		ctx = context.Background()
		log, _ = logger.NewZapLogger("debug", "text", zap.StacktraceLevel(zapcore.PanicLevel))

		cfg = &config.VPNClient{
			IsShootClient:  true,
			VPNNetwork:     network.CIDR(constants.DefaultVPNNetwork),
			VPNClientIndex: 0,
			HAVPNServers:   uint(2),
			BondingMode:    constants.BondingModeActiveBackup,
		}

		_ = exec.Command("mkdir", "-p", "/dev/net").Run()
		_ = exec.Command("mknod", "/dev/net/tun", "c", "10", "200").Run()
	})

	Context("when configuring bonding with active-backup mode for shoot client", func() {
		It("should successfully create bond with tap devices", func() {
			err := ConfigureBonding(ctx, log, cfg)
			Expect(err).NotTo(HaveOccurred())

			// Verify bond device was created
			bond, err := netlink.LinkByName(constants.BondDevice)
			Expect(err).NotTo(HaveOccurred())
			Expect(bond).NotTo(BeNil())

			// Verify tap devices were created and enslaved
			for i := 0; i < 2; i++ {
				tapName := "tap" + string(rune(48+i))
				tap, err := netlink.LinkByName(tapName)
				Expect(err).NotTo(HaveOccurred())
				Expect(tap.Attrs().MasterIndex).To(Equal(bond.Attrs().Index))
			}
		})

		It("should set correct bond mode attributes", func() {
			err := ConfigureBonding(ctx, log, cfg)
			Expect(err).NotTo(HaveOccurred())

			bond, err := netlink.LinkByName(constants.BondDevice)
			Expect(err).NotTo(HaveOccurred())

			bondLink := bond.(*netlink.Bond)
			Expect(bondLink.Mode).To(Equal(netlink.BOND_MODE_ACTIVE_BACKUP))
			Expect(bondLink.Miimon).To(Equal(100))
		})

		It("should assign ip address to bond device", func() {
			err := ConfigureBonding(ctx, log, cfg)
			Expect(err).NotTo(HaveOccurred())

			bond, err := netlink.LinkByName(constants.BondDevice)
			Expect(err).NotTo(HaveOccurred())

			addrs, err := netlink.AddrList(bond, netlink.FAMILY_V6)
			Expect(err).NotTo(HaveOccurred())
			Expect(addrs).NotTo(BeEmpty())
		})
	})

	Context("when configuring bonding with balance-rr mode", func() {
		BeforeEach(func() {
			cfg.BondingMode = constants.BondingModeBalanceRR
		})

		It("should successfully create bond in balance-rr mode", func() {
			err := ConfigureBonding(ctx, log, cfg)
			Expect(err).NotTo(HaveOccurred())

			bond, err := netlink.LinkByName(constants.BondDevice)
			Expect(err).NotTo(HaveOccurred())

			bondLink := bond.(*netlink.Bond)
			Expect(bondLink.Mode).To(Equal(netlink.BOND_MODE_BALANCE_RR))
		})
	})

	Context("when given unsupported bonding mode", func() {
		BeforeEach(func() {
			cfg.BondingMode = "invalid-mode"
		})

		It("should return error", func() {
			err := ConfigureBonding(ctx, log, cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported bonding mode"))
		})
	})

	Context("when bond device already exists", func() {
		BeforeEach(func() {
			// Pre-create bond device
			linkAttrs := netlink.NewLinkAttrs()
			bond := netlink.NewLinkBond(linkAttrs)
			bond.Name = constants.BondDevice
			err := netlink.LinkAdd(bond)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete and recreate bond device", func() {
			err := ConfigureBonding(ctx, log, cfg)
			Expect(err).NotTo(HaveOccurred())

			bond, err := netlink.LinkByName(constants.BondDevice)
			Expect(err).NotTo(HaveOccurred())
			Expect(bond).NotTo(BeNil())
		})
	})
})
