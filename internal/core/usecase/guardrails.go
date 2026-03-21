package usecase

import (
	"fmt"
	"regexp"
)

var denyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)rm\s+-rf\s+/`),
	regexp.MustCompile(`(?i)curl\s+.*\|\s*(ba)?sh`),
	regexp.MustCompile(`(?i)wget\s+.*\|\s*(ba)?sh`),
	regexp.MustCompile(`:\(\)\{.*:\|:&.*\};:`),
	regexp.MustCompile(`(?i)/etc/(passwd|shadow)`),
	regexp.MustCompile(`(?i)\b(shutdown|reboot|halt|poweroff)\b`),
	regexp.MustCompile(`(?i)\b(mkfs|fdisk)\b`),
	regexp.MustCompile(`(?i)\bdd\s+if=`),
}

// checkCodeSafety returns an error if the code contains dangerous patterns.
func checkCodeSafety(code string) error {
	for _, pattern := range denyPatterns {
		if pattern.MatchString(code) {
			return fmt.Errorf("code execution blocked: potentially dangerous pattern detected")
		}
	}
	return nil
}
