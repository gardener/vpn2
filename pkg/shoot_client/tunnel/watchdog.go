// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tunnel

import "fmt"

// Watchdog records failure and success events over a sliding window.
// Based on a configured threshold it can call an action if the failure threshold is breached.
// After the action has been called it won't be called again until cooldown has expired.
type Watchdog struct {
	window        []bool
	windowPos     int
	threshold     int
	cooldown      int
	cooldownCount int
	action        func() error
}

func NewWatchdog(windowsize, threshold, cooldown int, action func() error) (*Watchdog, error) {
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
	// Record failure in window
	wd.window[wd.windowPos] = failure
	wd.windowPos = (wd.windowPos + 1) % len(wd.window)

	// Reduce cooldown if still active
	if wd.cooldownCount > 0 {
		wd.cooldownCount--
	}

	// Count the number of failures in the window.
	failures := 0
	for _, failure := range wd.window {
		if failure {
			failures++
		}
	}

	// If the number of failures exceeds the threshold, call the action, reset cooldown
	if failures >= wd.threshold {
		if wd.cooldownCount == 0 {
			wd.cooldownCount = wd.cooldown
			return wd.action()
		}
	}

	return nil
}
