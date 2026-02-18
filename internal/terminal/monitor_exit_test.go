package terminal

import (
	"strings"
	"testing"
)

// Copy of detectExitCode logic from monitor.go to test it locally.
func detectExitCode(output string) int {
	errorIndicators := []string{
		"command not found",
		"No such file or directory",
		"Permission denied",
		"Invalid argument",
		"Operation not permitted",
		"syntax error",
		"cannot access",
		"not recognized",
	}

	lowerOutput := strings.ToLower(output)
	for _, indicator := range errorIndicators {
		if strings.Contains(lowerOutput, strings.ToLower(indicator)) {
			return 1
		}
	}
	return 0
}

func TestAptUpdate_ExitCodeDetection(t *testing.T) {
	output := `Get:1 http://security.ubuntu.com/ubuntu jammy-security InRelease [129 kB]
Get:2 http://archive.ubuntu.com/ubuntu jammy InRelease [270 kB]
Fetched 44.0 MB in 32s (1390 kB/s)                                                                                      
Reading package lists... Done
Building dependency tree... Done
Reading state information... Done
6 packages can be upgraded. Run 'apt list --upgradable' to see them.`

	code := detectExitCode(output)
	if code != 0 {
		t.Errorf("Expected ExitCode 0 (Success) for apt output, got %d", code)
	}
}
