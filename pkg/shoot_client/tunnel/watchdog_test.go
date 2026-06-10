// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Watchdog", func() {
	Describe("NewWatchdog", func() {
		It("returns a valid watchdog with correct parameters", func() {
			wd, err := NewWatchdog(5, 3, 2, func() error { return nil })
			Expect(err).NotTo(HaveOccurred())
			Expect(wd).NotTo(BeNil())
			Expect(wd.window).To(HaveLen(5))
			Expect(wd.threshold).To(Equal(3))
			Expect(wd.cooldown).To(Equal(2))
			Expect(wd.cooldownCount).To(Equal(0))
		})

		It("returns an error when windowsize is zero", func() {
			_, err := NewWatchdog(0, 1, 1, func() error { return nil })
			Expect(err).To(MatchError(ContainSubstring("invalid windowsize")))
		})

		It("returns an error when windowsize is negative", func() {
			_, err := NewWatchdog(-1, 1, 1, func() error { return nil })
			Expect(err).To(MatchError(ContainSubstring("invalid windowsize")))
		})

		It("returns an error when threshold is zero", func() {
			_, err := NewWatchdog(5, 0, 1, func() error { return nil })
			Expect(err).To(MatchError(ContainSubstring("invalid threshold")))
		})

		It("returns an error when threshold is negative", func() {
			_, err := NewWatchdog(5, -1, 1, func() error { return nil })
			Expect(err).To(MatchError(ContainSubstring("invalid threshold")))
		})

		It("returns an error when threshold exceeds windowsize", func() {
			_, err := NewWatchdog(3, 5, 1, func() error { return nil })
			Expect(err).To(MatchError(ContainSubstring("must be <= windowsize")))
		})

		It("returns an error when cooldown is zero", func() {
			_, err := NewWatchdog(5, 3, 0, func() error { return nil })
			Expect(err).To(MatchError(ContainSubstring("invalid cooldown")))
		})

		It("returns an error when cooldown is negative", func() {
			_, err := NewWatchdog(5, 3, -1, func() error { return nil })
			Expect(err).To(MatchError(ContainSubstring("invalid cooldown")))
		})

		It("allows threshold equal to windowsize", func() {
			wd, err := NewWatchdog(5, 5, 1, func() error { return nil })
			Expect(err).NotTo(HaveOccurred())
			Expect(wd).NotTo(BeNil())
		})
	})

	Describe("Fail and Succeed", func() {
		var (
			actionCount int
			actionErr   error
			wd          *Watchdog
		)

		BeforeEach(func() {
			actionCount = 0
			actionErr = nil
		})

		JustBeforeEach(func() {
			var err error
			// windowsize=5, threshold=3, cooldown=3
			wd, err = NewWatchdog(5, 3, 3, func() error {
				actionCount++
				return actionErr
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not trigger action when failures are below threshold", func() {
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(actionCount).To(Equal(0))
		})

		It("triggers action when failures reach the threshold", func() {
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(actionCount).To(Equal(1))
		})

		It("does not trigger action on successes", func() {
			Expect(wd.Succeed()).To(Succeed())
			Expect(wd.Succeed()).To(Succeed())
			Expect(wd.Succeed()).To(Succeed())
			Expect(actionCount).To(Equal(0))
		})

		It("does not re-trigger action during cooldown period", func() {
			// Trigger the action once
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(actionCount).To(Equal(1))

			// Additional failures during cooldown should not trigger again
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(actionCount).To(Equal(1))
		})

		It("re-triggers action after cooldown expires", func() {
			// Trigger the action once; cooldown=3
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(actionCount).To(Equal(1))

			// Drain cooldown with 3 more records (each decrements cooldownCount)
			Expect(wd.Fail()).To(Succeed()) // cooldownCount: 3->2
			Expect(wd.Fail()).To(Succeed()) // cooldownCount: 2->1
			Expect(wd.Fail()).To(Succeed()) // cooldownCount: 1->0

			// Now cooldown is expired; next failing record should trigger again
			Expect(wd.Fail()).To(Succeed())
			Expect(actionCount).To(Equal(2))
		})

		It("resets failures after enough successes slide out the window", func() {
			// Fill window with failures to trigger action
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(actionCount).To(Equal(1))

			// Slide out old failures with successes (windowsize=5, need 5 successes)
			for i := 0; i < 5; i++ {
				Expect(wd.Succeed()).To(Succeed())
			}

			// Action count stays the same (all successes now)
			Expect(actionCount).To(Equal(1))
		})

		It("returns the action error when the action fails", func() {
			actionErr = fmt.Errorf("action failed")
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			err := wd.Fail()
			Expect(err).To(MatchError("action failed"))
		})

		It("wraps window correctly and counts failures in a circular manner", func() {
			// windowsize=5, threshold=3
			// Record 2 failures, then 3 successes (pushing failures out), then 3 more failures
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Succeed()).To(Succeed())
			Expect(wd.Succeed()).To(Succeed())
			Expect(wd.Succeed()).To(Succeed())
			Expect(actionCount).To(Equal(0))
			// Now 5 records: F F S S S -> 2 failures, no trigger

			// 3 more failures slide out old ones: F F S S S | F -> window: F S S S F (2 failures)
			// F F S S S | F F -> window: S S S F F (2 failures)
			// F F S S S | F F F -> window: S S F F F (3 failures) -> trigger
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(wd.Fail()).To(Succeed())
			Expect(actionCount).To(Equal(1))
		})
	})
})
