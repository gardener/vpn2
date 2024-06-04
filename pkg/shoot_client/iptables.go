package shoot_client

import (
	"github.com/coreos/go-iptables/iptables"
	"github.com/gardener/vpn2/pkg/config"
)

func SetIPTableRules(cfg config.ShootClient) error {
	forwardDevice := "tun0"
	if cfg.VPNServerIndex != "" {
		forwardDevice = "bond0"
	}

	protocol := iptables.ProtocolIPv4
	if cfg.IPFamilies == "IPv6" {
		protocol = iptables.ProtocolIPv6
	}
	iptable, err := iptables.New(iptables.IPFamily(protocol))
	if err != nil {
		return err
	}

	if cfg.IsShootClient {
		if cfg.IPFamilies == "IPv4" {
			err = iptable.Append("filter", "FORWARD", "--in-interface", forwardDevice, "-j", "ACCEPT")
			if err != nil {
				return err
			}
		}

		err = iptable.Append("nat", "POSTROUTING", "--out-interface", "eth0", "-j", "MASQUERADE")
		if err != nil {
			return err
		}
	} else {
		err = iptable.AppendUnique("filter", "INPUT", "-m", "state", "--state", "RELATED,ESTABLISHED", "-i", forwardDevice, "-j", "ACCEPT")
		if err != nil {
			return err
		}
		err = iptable.AppendUnique("filter", "INPUT", "-i", forwardDevice, "-j", "DROP")
		if err != nil {
			return err
		}
	}
	return nil
}
