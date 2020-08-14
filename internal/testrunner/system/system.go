package system

type runner struct {
	testFolderPath string
}

// Run runs the system tests defined under the given folder
func Run(testFolderPath string) error {
	r := runner{testFolderPath}
	return r.run()
}

func (r *runner) run() error {
	return nil
}
