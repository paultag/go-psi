package psi

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Resource to monitor PSI backpressure on.
type Resource string

var (
	ResourceCPU    Resource = "cpu"
	ResourceIO     Resource = "io"
	ResourceMemory Resource = "memory"
)

// StallType represents how we measure "stall" during the time window.
type StallType string

var (
	StallTypeFull StallType = "full"
	StallTypeSome StallType = "some"
)

type Config struct {
	Resource            Resource
	Type                StallType
	StallWindowDuration time.Duration
	WindowDuration      time.Duration
}

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

func (c Config) Explain() string {
	var name string = "UNKNOWN"
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

//
//
//
func Monitor(
	config Config,
	cb func() error,
) error {
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
			return err
		}
	}
}
