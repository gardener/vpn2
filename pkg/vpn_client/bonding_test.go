package vpn_client

import (
	"context"
	"fmt"
	"net"
	"os/exec"

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

func hasV6Route(link netlink.Link, dst *net.IPNet) bool {
	routes, err := netlink.RouteList(link, netlink.FAMILY_V6)
	if err != nil {
		return false
	}
	for _, route := range routes {
		if route.Dst != nil && route.Dst.String() == dst.String() {
			return true
		}
	}
	return false
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
			HAVPNClients:   uint(2),
			BondingMode:    constants.BondingModeActiveBackup,
		}

		err := EnableIPv6Networking(log)
		Expect(err).NotTo(HaveOccurred())
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
			for i := range cfg.HAVPNServers {
				tapName := fmt.Sprintf("tap%d", i)
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

		It("should add route from shoot bonding subnet to seed bonding subnet", func() {
			err := ConfigureBonding(ctx, log, cfg)
			Expect(err).NotTo(HaveOccurred())

			bond, err := netlink.LinkByName(constants.BondDevice)
			Expect(err).NotTo(HaveOccurred())

			seedBase, _, _ := network.BondingSeedClientRange(cfg.VPNNetwork.ToIPNet().IP)
			seedSubnet := network.BondingAddressForClient(seedBase)
			Expect(hasV6Route(bond, seedSubnet)).To(BeTrue())
		})
	})

	Context("when configuring bonding for seed client", func() {
		BeforeEach(func() {
			cfg.IsShootClient = false
			cfg.PodName = "seed-client-test-0"
			cfg.HAVPNClients = 0 // keep test focused on route installation only
		})

		It("should add route from seed bonding subnet to shoot bonding subnet", func() {
			err := ConfigureBonding(ctx, log, cfg)
			Expect(err).NotTo(HaveOccurred())

			bond, err := netlink.LinkByName(constants.BondDevice)
			Expect(err).NotTo(HaveOccurred())

			shootSubnet := network.BondingShootClientAddress(cfg.VPNNetwork.ToIPNet(), 0)
			Expect(hasV6Route(bond, shootSubnet)).To(BeTrue())
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
