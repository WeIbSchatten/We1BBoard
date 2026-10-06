package supervisor

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Process manages a long-running child binary.
type Process struct {
	Name    string
	Bin     string
	Args    []string
	WorkDir string
	Env     []string

	// OnExit is called (async) when the process exits. May be nil.
	OnExit func(err error)

	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
	started time.Time
	logW    io.Writer
	lastErr error
	done    chan struct{} // closed once Wait() finishes; never call Process.Wait twice
}

func New(name, bin string, args ...string) *Process {
	return &Process{Name: name, Bin: bin, Args: args, logW: os.Stdout}
}

func (p *Process) SetLog(w io.Writer) { p.logW = w }

func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// LastExitError returns the last Wait() error (nil if clean exit / never started).
func (p *Process) LastExitError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastErr
}

func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return nil
	}
	// Refuse to start while a previous Wait() is still finishing.
	if p.done != nil {
		select {
		case <-p.done:
		default:
			return fmt.Errorf("%s previous process still stopping — retry", p.Name)
		}
	}
	if _, err := os.Stat(p.Bin); err != nil {
		return fmt.Errorf("%s binary not found at %s: %w", p.Name, p.Bin, err)
	}
	cmd := exec.Command(p.Bin, p.Args...)
	if p.WorkDir != "" {
		cmd.Dir = p.WorkDir
	}
	if len(p.Env) > 0 {
		cmd.Env = append(os.Environ(), p.Env...)
	}
	cmd.Stdout = p.logW
	cmd.Stderr = p.logW
	if err := cmd.Start(); err != nil {
		return err
	}
	p.cmd = cmd
	p.running = true
	p.started = time.Now()
	p.lastErr = nil
	done := make(chan struct{})
	p.done = done
	onExit := p.OnExit
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.running = false
		p.lastErr = err
		p.mu.Unlock()
		close(done)
		if err != nil {
			log.Printf("[%s] exited: %v", p.Name, err)
		} else {
			log.Printf("[%s] exited cleanly", p.Name)
		}
		if onExit != nil {
			onExit(err)
		}
	}()
	log.Printf("[%s] started pid=%d", p.Name, cmd.Process.Pid)
	return nil
}

// StartAndWaitHealthy starts the process and waits up to wait for it to stay alive.
func (p *Process) StartAndWaitHealthy(wait time.Duration) error {
	if err := p.Start(); err != nil {
		return err
	}
	if wait <= 0 {
		wait = 1500 * time.Millisecond
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if !p.IsRunning() {
			err := p.LastExitError()
			if err != nil {
				return fmt.Errorf("%s exited immediately: %w", p.Name, err)
			}
			return fmt.Errorf("%s exited immediately (check process.log / error.log)", p.Name)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !p.IsRunning() {
		err := p.LastExitError()
		if err != nil {
			return fmt.Errorf("%s exited immediately: %w", p.Name, err)
		}
		return fmt.Errorf("%s exited immediately (check process.log / error.log)", p.Name)
	}
	return nil
}

func (p *Process) Stop() error {
	p.mu.Lock()
	cmd := p.cmd
	done := p.done
	running := p.running
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
		return nil
	}
	if !running {
		// Already exiting — still wait for Wait() so the port is freed.
		if done != nil {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
			}
		}
		return nil
	}

	_ = p.signalStop(cmd)

	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
			}
		}
	}

	p.mu.Lock()
	p.running = false
	p.mu.Unlock()
	// Brief grace so the kernel releases listen sockets before a restart.
	time.Sleep(300 * time.Millisecond)
	return nil
}

func (p *Process) Restart() error {
	_ = p.Stop()
	return p.Start()
}

// RestartAndWaitHealthy restarts and verifies the process stays up.
func (p *Process) RestartAndWaitHealthy(wait time.Duration) error {
	_ = p.Stop()
	return p.StartAndWaitHealthy(wait)
}

func (p *Process) Uptime() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return 0
	}
	return time.Since(p.started)
}
