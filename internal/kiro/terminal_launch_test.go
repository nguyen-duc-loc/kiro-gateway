package kiro

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

var terminalLaunchCheck = flag.Bool("terminal-launch-check", false, "explicit local terminal fixture, no account access")
var terminalChildHold = flag.Bool("terminal-child-hold", false, "hold the synthetic terminal child for cancellation")
var terminalInputChild = flag.Bool("terminal-input-child", false, "local terminal fixture child")

// runInteractiveClient retains one child process group for bounded cancellation.
func runInteractiveClient(cmd *exec.Cmd) (err error) {
	terminal := os.Stdin.Fd()
	var previous int32
	if err := terminalProcessGroup(terminal, syscall.TIOCGPGRP, &previous); err != nil {
		return errors.New("interactive client requires a controlling terminal")
	}
	// Foreground retains the child's separate group for cancellation while
	// allowing terminal input. Setpgid alone leaves it subject to SIGTTIN.
	cmd.SysProcAttr = &syscall.SysProcAttr{Foreground: true, Ctty: int(terminal)}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	defer func() {
		// The runner is temporarily in the background when it takes the
		// terminal back. Ignore SIGTTOU only across that terminal operation.
		ignored := signal.Ignored(syscall.SIGTTOU)
		signal.Ignore(syscall.SIGTTOU)
		restore := terminalProcessGroup(terminal, syscall.TIOCSPGRP, &previous)
		if !ignored {
			signal.Reset(syscall.SIGTTOU)
		}
		if restore != nil {
			err = errors.New("could not restore the controlling terminal")
		}
	}()
	return cmd.Run()
}

func terminalProcessGroup(fd, operation uintptr, group *int32) error {
	// Darwin's terminal ioctls take a pointer to a pid_t (32 bit signed int).
	// The pointer is passed directly for this synchronous system call only.
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, operation, uintptr(unsafe.Pointer(group)))
	if errno != 0 {
		return errno
	}
	return nil
}

func TestInteractiveClientTerminalInput(t *testing.T) {
	if !*terminalLaunchCheck {
		t.Skip("requires an explicit controlling terminal fixture")
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestTerminalInputChild$", "-terminal-input-child")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := runInteractiveClient(cmd); err != nil {
		t.Fatalf("interactive child could not read its terminal: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("interactive child reached deadline")
	}
	var restored int32
	if err := terminalProcessGroup(os.Stdin.Fd(), syscall.TIOCGPGRP, &restored); err != nil || int(restored) != syscall.Getpgrp() {
		t.Fatalf("foreground group not restored: error=%v", err)
	}
}

func TestTerminalInputChild(t *testing.T) {
	if !*terminalInputChild {
		t.Skip("terminal fixture child only")
	}
	fmt.Fprintln(os.Stdout, "terminal_fixture_ready")
	if *terminalChildHold {
		time.Sleep(time.Minute)
		return
	}
	value, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil || value != "fixture\n" {
		t.Fatal(errors.New("terminal fixture input not received"))
	}
}

func TestInteractiveClientTerminalCancellation(t *testing.T) {
	if !*terminalLaunchCheck {
		t.Skip("requires an explicit controlling terminal fixture")
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestTerminalInputChild$", "-terminal-input-child", "-terminal-child-hold")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	started := time.Now()
	err = runInteractiveClient(cmd)
	if err == nil || ctx.Err() != context.DeadlineExceeded || time.Since(started) > 5*time.Second {
		t.Fatalf("terminal cancellation err=%v context=%v, want deadline and joined child within five seconds", err, ctx.Err())
	}
	var restored int32
	if err := terminalProcessGroup(os.Stdin.Fd(), syscall.TIOCGPGRP, &restored); err != nil || int(restored) != syscall.Getpgrp() {
		t.Fatalf("cancellation did not restore foreground: error=%v", err)
	}
	if err := syscall.Kill(-cmd.Process.Pid, 0); err != syscall.ESRCH {
		t.Errorf("child group still exists after cancellation: %v", err)
	}
}
