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

	mu      sync.Mutex
	cmd     *exec.Cmd
	running bool
	started time.Time
	logW    io.Writer
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

func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return nil
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
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
		if err != nil {
			log.Printf("[%s] exited: %v", p.Name, err)
		} else {
			log.Printf("[%s] exited cleanly", p.Name)
		}
	}()
	log.Printf("[%s] started pid=%d", p.Name, cmd.Process.Pid)
	return nil
}

func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running || p.cmd == nil || p.cmd.Process == nil {
		p.running = false
		return nil
	}
	_ = p.stopOS()
	p.running = false
	return nil
}

func (p *Process) Restart() error {
	_ = p.Stop()
	time.Sleep(200 * time.Millisecond)
	return p.Start()
}

func (p *Process) Uptime() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return 0
	}
	return time.Since(p.started)
}
