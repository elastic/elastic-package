package shell

// Type is a proxy type for iota constants defining supported shell types.
// Shell types are used by to return valid shell code to be eval-ed in
// the corresponding shell to setup the environment required to connect
// to the stack setup by this package.
type Type int

// String returns the string representation of iota constants
func (s Type) String() string {
	return AvailableShellTypes()[s]
}

// FromString return the iota corresponding to it's string representation.
// In case the string is not one of the available types it defaults to Sh.
func FromString(s string) Type {
	switch s {
	case "bash":
		return Bash
	case "fish":
		return Fish
	case "sh":
		return Sh
	case "zsh":
		return Zsh
	default:
		return Sh
	}
}
