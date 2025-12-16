package vpn_client

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVPNClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VPN Client Suite")
}
