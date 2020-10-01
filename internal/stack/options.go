package stack

// Options defines available image booting options.
type Options struct {
	DaemonMode   bool
	StackVersion string

	Services []string
}