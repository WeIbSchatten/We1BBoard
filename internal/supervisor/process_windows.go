package supervisor

func (p *Process) stopOS() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	_ = p.cmd.Process.Kill()
	_, _ = p.cmd.Process.Wait()
	return nil
}
