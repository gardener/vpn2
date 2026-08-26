// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package pathcontroller

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPathcontroller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pathcontroller Suite")
}
