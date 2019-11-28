// {{{ Copyright (c) Paul R. Tagliamonte <paultag@gmail.com>, 2019
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE. }}}

package psi

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Resource to monitor PSI backpressure on. This is currently one of
// "cpu", "io" or "memory", as defined by ResourceCPU, ResourceIO and
// ResourceMemory, respectively.
type Resource string

var (
	// ResourceCPU represents the CPU when monitoring
	ResourceCPU Resource = "cpu"

	// ResourceIO represents IO when monitoring
	ResourceIO Resource = "io"

	// ResourceMemory represents memory when monitoring
	ResourceMemory Resource = "memory"
)

// StallType represents how we measure "stall" during the time window.
type StallType string

var (
	// StallTypeFull signifies all tasks need to have stalled for at least the
	// stall window duration during the window duration
	StallTypeFull StallType = "full"

	// StallTypeSome signifies at least one task must have stalled for at least
	// the stall window duration during the window duration.
	StallTypeSome StallType = "some"
)

// Config sets the parameters used to monitor backpressure on a resource.
type Config struct {
	Resource            Resource
	Type                StallType
	StallWindowDuration time.Duration
	WindowDuration      time.Duration
}

// Check that the values contained in the Config are valid for use to monitor
// Backpressure.
//
// In particular, this will check the range on provided WindowDuration and
// StallWindowDuration minimum and maximums.
//
// If you're programatically generating the struct, be sure to run `Check` on
// the values before using them to catch errors in a way that's a bit easier to
// reason about.
func (c Config) Check() error {
	if c.WindowDuration < time.Millisecond*500 {
		return fmt.Errorf("Minmum WindowDuration is 500ms")
	}
	if c.WindowDuration > time.Second*10 {
		return fmt.Errorf("Maximum WindowDuration is 10s")
	}

	if c.StallWindowDuration < time.Millisecond*50 {
		return fmt.Errorf("Minmum StallWindowDuration is 50ms")
	}
	if c.StallWindowDuration > time.Second {
		return fmt.Errorf("Maximum StallWindowDuration is 1s")
	}

	return nil
}

// Explain will return a human readable string explaining what the query will
// be triggering on.
func (c Config) Explain() string {
	name := "UNKNOWN"
	switch c.Type {
	case StallTypeSome:
		name = "at least one"
	case StallTypeFull:
		name = "all of"
	}

	return fmt.Sprintf(
		"%s of the tasks in the queue are waiting for %s for longer than %s measured within a %s time window\n",
		name,
		c.Resource,
		c.StallWindowDuration.String(),
		c.WindowDuration.String(),
	)
}

// MonitorCallback allows Monitor to invoke a callback when the backpressure
// exceeds the provided thresholds.
type MonitorCallback func() error

var (
	// ErrStopMonitoring allows `MonitorCallback`s to tell Monitor to bail
	// and stop paying attention.
	ErrStopMonitoring error = fmt.Errorf("psi: stop it")
)

// Monitor will invoke the provided Callback every time the backpressure
// thresholds exceed the provided configuration.
func Monitor(config Config, cb MonitorCallback) error {
	if err := config.Check(); err != nil {
		return err
	}

	fd, err := os.OpenFile(
		fmt.Sprintf("/proc/pressure/%s", config.Resource),
		syscall.O_RDWR|syscall.O_NONBLOCK,
		0,
	)
	if err != nil {
		return err
	}
	defer fd.Close()

	_, err = fmt.Fprintf(
		fd,
		"%s %d %d\x00",
		config.Type,
		config.StallWindowDuration.Microseconds(),
		config.WindowDuration.Microseconds(),
	)
	if err != nil {
		return err
	}

	for {
		_, err := unix.Poll([]unix.PollFd{unix.PollFd{
			Fd:     int32(fd.Fd()),
			Events: syscall.EPOLLPRI,
		}}, -1)
		if err != nil {
			return err
		}
		if err := cb(); err != nil {
			if err == ErrStopMonitoring {
				break
			}
			return err
		}
	}
	return nil
}

// vim: foldmethod=marker
