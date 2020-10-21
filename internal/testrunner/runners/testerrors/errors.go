package testerrors

import "fmt"

// ErrTestCaseFailed represents a test case failure result
type ErrTestCaseFailed struct {
	Reason  string
	Details string
}

// Error returns the message detailing the test case failure.
func (e ErrTestCaseFailed) Error() string {
	return fmt.Sprintf("test case failed: %s", e.Reason)
}
