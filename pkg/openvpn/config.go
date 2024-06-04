package openvpn

import (
	"io"
	"net"
	"text/template"

	"github.com/gardener/vpn2/pkg/network"
)

const defaultOpenVPNConfigFile = "/openvpn.config"

func executeTemplate(name string, w io.Writer, templt string, data any) error {
	cidrMaskFunc :=
		func(n network.CIDR) string {
			mask := net.CIDRMask(n.Mask.Size())
			return net.IPv4(255, 255, 255, 255).Mask(mask).String()
		}

	var funcs = map[string]any{"cidrMask": cidrMaskFunc}
	t, err := template.New(name).
		Funcs(funcs).
		Parse(templt)
	if err != nil {
		return err
	}
	return t.Execute(w, data)
}
