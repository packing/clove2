package base

import (
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// Waiting for a signal for the process to exit or interrupt
func WaitExitSignal() os.Signal {
	c := make(chan os.Signal, 1)
	signals := []os.Signal{
		os.Interrupt,
		os.Kill,
		syscall.SIGTERM,
		syscall.SIGKILL,
		syscall.Signal(0x11),
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.Signal(0x1e),
		syscall.Signal(0x1f),
	}
	signal.Notify(c, signals...)
	s := <-c
	LogVerbose(">>> 收到信号 > %s", s.String())
	return s
}

// Get the ID of the current Goroutine
func GetGoroutineId() int {
	defer func() {
		if err := recover(); err != nil {
			LogError("An error occurred getting the Goroutine ID.", err)
		}
	}()

	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		LogError("Invalid Goroutine ID was obtained.", err)
		return -1
	}
	return id
}
