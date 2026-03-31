package hook

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	hookMarkerStart = "# >>> ledger hook >>>"
	hookMarkerEnd   = "# <<< ledger hook <<<"
)

// FindGitDir finds the .git directory from the given base directory.
func FindGitDir(baseDir string) (string, error) {
	out, err := exec.Command("git", "-C", baseDir, "rev-parse", "--git-dir").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	gitDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(baseDir, gitDir)
	}
	return gitDir, nil
}

func hookPath(gitDir string) string {
	return filepath.Join(gitDir, "hooks", "post-commit")
}

func ledgerBlock(ledgerBin string) string {
	return fmt.Sprintf(`%s
%s index --quiet 2>/dev/null &
%s`, hookMarkerStart, ledgerBin, hookMarkerEnd)
}

// Install adds a ledger post-commit hook.
// If a post-commit hook already exists, the ledger block is appended.
func Install(baseDir string) error {
	gitDir, err := FindGitDir(baseDir)
	if err != nil {
		return err
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	hp := hookPath(gitDir)

	// ledger 바이너리 경로 결정
	ledgerBin, err := os.Executable()
	if err != nil {
		ledgerBin = "ledger"
	}

	existing, _ := os.ReadFile(hp)
	content := string(existing)

	// 이미 설치되어 있으면 스킵
	if strings.Contains(content, hookMarkerStart) {
		return fmt.Errorf("ledger hook already installed")
	}

	block := ledgerBlock(ledgerBin)

	if len(existing) == 0 {
		// 새 파일 생성
		content = "#!/bin/sh\n" + block + "\n"
	} else {
		// 기존 hook에 추가
		content = content + "\n" + block + "\n"
	}

	if err := os.WriteFile(hp, []byte(content), 0755); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}

	return nil
}

// Uninstall removes the ledger block from post-commit hook.
func Uninstall(baseDir string) error {
	gitDir, err := FindGitDir(baseDir)
	if err != nil {
		return err
	}

	hp := hookPath(gitDir)
	data, err := os.ReadFile(hp)
	if err != nil {
		return fmt.Errorf("no post-commit hook found")
	}

	content := string(data)
	if !strings.Contains(content, hookMarkerStart) {
		return fmt.Errorf("ledger hook not installed")
	}

	// 마커 사이 블록 제거
	startIdx := strings.Index(content, hookMarkerStart)
	endIdx := strings.Index(content, hookMarkerEnd)
	if startIdx == -1 || endIdx == -1 {
		return fmt.Errorf("malformed hook markers")
	}

	before := content[:startIdx]
	after := content[endIdx+len(hookMarkerEnd):]

	cleaned := strings.TrimRight(before, "\n") + after
	cleaned = strings.TrimSpace(cleaned)

	// hook이 shebang만 남으면 파일 삭제
	if cleaned == "#!/bin/sh" || cleaned == "" {
		return os.Remove(hp)
	}

	return os.WriteFile(hp, []byte(cleaned+"\n"), 0755)
}

// Status returns whether the ledger hook is installed.
func Status(baseDir string) (installed bool, hookFile string) {
	gitDir, err := FindGitDir(baseDir)
	if err != nil {
		return false, ""
	}

	hp := hookPath(gitDir)
	data, err := os.ReadFile(hp)
	if err != nil {
		return false, hp
	}

	return strings.Contains(string(data), hookMarkerStart), hp
}
