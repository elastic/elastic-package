package shell

// AvailableShellTypes returns all available shell types in human format
func AvailableShellTypes() []string {
	return []string{"bash", "fish", "sh", "zsh"}
}

const (
	// Bash Type constant
	Bash Type = iota
	// Fish Type constant
	Fish
	// Sh Type constant
	Sh
	// Zsh Type constant
	Zsh
)
