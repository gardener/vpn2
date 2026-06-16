// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tunnel

import (
	"fmt"

	"github.com/go-logr/logr"
)

// Watchdog records failure and success events over a sliding window.
// Based on a configured threshold it can call an action if the failure threshold is breached.
// After the action has been called, the window is reset and no events are recorded until a cooldown has expired.
type Watchdog struct {
	log           logr.Logger
	window        []bool
	windowPos     int
	threshold     int
	cooldown      int
	cooldownCount int
	action        func() error
}

func NewWatchdog(log logr.Logger, windowsize, threshold, cooldown int, action func() error) (*Watchdog, error) {
	log.Info("watchdog initialized", "windowSize", windowsize, "threshold", threshold, "cooldown", cooldown)
	if windowsize <= 0 {
		return nil, fmt.Errorf("invalid windowsize %d, must be > 0", windowsize)
	}

	if threshold <= 0 {
		return nil, fmt.Errorf("invalid threshold %d, must be > 0", threshold)
	}

	if threshold > windowsize {
		return nil, fmt.Errorf("invalid threshold %d, must be <= windowsize %d", threshold, windowsize)
	}

	if cooldown <= 0 {
		return nil, fmt.Errorf("invalid cooldown %d, must be > 0", cooldown)
	}

	wd := &Watchdog{
		log:           log,
		window:        make([]bool, windowsize),
		windowPos:     0,
		threshold:     threshold,
		cooldown:      cooldown,
		cooldownCount: 0,
		action:        action,
	}

	return wd, nil
}

func (wd *Watchdog) Fail() error {
	return wd.record(true)
}

func (wd *Watchdog) Succeed() error {
	return wd.record(false)
}

func (wd *Watchdog) record(failure bool) error {
	// Reduce cooldown if still active
	if wd.cooldownCount > 0 {
		wd.cooldownCount--
		if failure {
			wd.log.Info("watchdog", "cooldown", wd.cooldownCount)
		}
		return nil
	}

	// Record failure in window
	wd.window[wd.windowPos] = failure
	wd.windowPos = (wd.windowPos + 1) % len(wd.window)

	// Count the number of failures in the window.
	failures := 0
	for _, failure := range wd.window {
		if failure {
			failures++
		}
	}

	if failure {
		wd.log.Info("watchdog", "failures", failures, "threshold", wd.threshold)
	}

	// If the number of failures exceeds the threshold, call the action, reset the window and start the cooldown.
	if failures >= wd.threshold {
		wd.cooldownCount = wd.cooldown
		wd.reset()
		return wd.action()
	}

	return nil
}

func (wd *Watchdog) reset() {
	wd.window = make([]bool, len(wd.window))
	wd.windowPos = 0
}
