package system

type runner struct {
	systemTestsPath string
}

func newRunner(systemTestsPath string) (*runner, error) {
	return &runner{systemTestsPath}, nil
}

func (r *runner) run() error {
	return nil
}
