// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package openvpn

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func TestOpenVPN(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OpenVPN Suite")
}

// OpenVPNConfigMaxLineLength is the maximum length of a line in an OpenVPN configuration file.
// see https://github.com/OpenVPN/openvpn/blob/master/src/openvpn/options.h#L58
// With dual-stack shoots plus the seed pod network, the configuration can get quite long so we have to check for this.
const OpenVPNConfigMaxLineLength = 256

func HaveNoLineLongerThan(maxLength int) types.GomegaMatcher {
	return &noLineLongerThanMatcher{maxLength: maxLength}
}

type noLineLongerThanMatcher struct {
	maxLength int
}

func (matcher *noLineLongerThanMatcher) Match(actual interface{}) (success bool, err error) {
	content, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("HaveNoLineLongerThan matcher expects a string")
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if len(line) > matcher.maxLength {
			return false, nil
		}
	}
	return true, nil
}

func (matcher *noLineLongerThanMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%s\nto have no line longer than %d characters", actual, matcher.maxLength)
}

func (matcher *noLineLongerThanMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%s\nto have at least one line longer than %d characters", actual, matcher.maxLength)
}
