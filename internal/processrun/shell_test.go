package processrun

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The real script runs only against a temporary workspace and a stub SVN.
// No real repository or Unity installation is touched.
func TestBuildScriptPropagatesFailures(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("shell unavailable")
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "shell", "autoBuildClient.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, svn string
		code      int
	}{
		{"unity failure", "exit 0", 7},
		{"svn failure", "exit 5", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, "bin")
			project := filepath.Join(root, "project with spaces")
			engine := filepath.Join(root, "engine with spaces")
			unityDir := filepath.Join(engine, "Unity.app", "Contents", "MacOS")
			for _, dir := range []string{bin, project, unityDir} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(bin, "svn"), []byte("#!/bin/sh\n"+tc.svn+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(unityDir, "Unity"), []byte("#!/bin/sh\nexit 7\n"), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(sh, script, engine, project, "method", "", "", "", "", "")
			// Forward slashes work as Git Bash paths and POSIX PATH entries.
			cmd.Env = append(os.Environ(), "PATH="+filepath.ToSlash(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			if err == nil || cmd.ProcessState.ExitCode() != tc.code {
				t.Fatalf("exit=%v wanted=%d output=%s", err, tc.code, out)
			}
		})
	}
}
