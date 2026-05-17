package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBuildTarStreamArgs_BackslashDstBecomesForwardSlash locks in the
// rule that the remote tar command must receive a forward-slash path
// for -C. We hit this regression three times in one session — a
// backslash path made bsdtar print "could not chdir" and the tar pipe
// hung forever. Test guards against re-introduction.
func TestBuildTarStreamArgs_BackslashDstBecomesForwardSlash(t *testing.T) {
	_, remoteCmd := buildTarStreamArgs("/home/x/repo", `C:\maldev`)
	assert.Contains(t, remoteCmd, "-C C:/maldev",
		"-C path must be forward-slashed for bsdtar on Windows")
	assert.NotContains(t, remoteCmd, `C:\maldev`,
		"backslash form must not survive into the remote command")
}

// TestBuildTarStreamArgs_ForwardSlashDstPassesThrough confirms an
// already-normalised path round-trips unchanged.
func TestBuildTarStreamArgs_ForwardSlashDstPassesThrough(t *testing.T) {
	_, remoteCmd := buildTarStreamArgs("/home/x/repo", "C:/maldev")
	assert.Contains(t, remoteCmd, "-C C:/maldev")
}

// TestBuildTarStreamArgs_RemoteCmdHasNoCmdExeWrapper guards against
// reintroducing the `cmd.exe /c "..."` wrapper. Windows OpenSSH
// already wraps remote commands in cmd.exe, so a nested wrapper
// fragments any embedded `&` and silently breaks the path-creation
// chain.
func TestBuildTarStreamArgs_RemoteCmdHasNoCmdExeWrapper(t *testing.T) {
	_, remoteCmd := buildTarStreamArgs("/home/x/repo", `C:\maldev`)
	assert.False(t, strings.Contains(remoteCmd, "cmd.exe"),
		"remote command must not nest a cmd.exe wrapper: %q", remoteCmd)
}

// TestBuildTarStreamArgs_ContainsCanonicalExcludes guards the exclude
// list against accidental shrinkage. The killer one is `ignore` — its
// removal would silently ship multi-hundred-MB sub-trees on every
// libvirt Windows run. Other excludes follow the rsync set in
// pushLinux for symmetry.
func TestBuildTarStreamArgs_ContainsCanonicalExcludes(t *testing.T) {
	tarArgs, _ := buildTarStreamArgs("/home/x/repo", `C:\maldev`)
	must := []string{"--exclude=ignore", "--exclude=.git", "--exclude=.claude"}
	for _, want := range must {
		assert.Contains(t, tarArgs, want, "exclude list missing %q", want)
	}
}

// TestBuildTarStreamArgs_TarArgsShape locks the overall argv shape so
// future contributors don't accidentally drop the trailing `.` (which
// tells tar to package the current directory after `-C`).
func TestBuildTarStreamArgs_TarArgsShape(t *testing.T) {
	tarArgs, _ := buildTarStreamArgs("/home/x/repo", `C:\maldev`)
	require := assert.New(t)
	require.Equal("-cf", tarArgs[0])
	require.Equal("-", tarArgs[1])
	// Last three args: "-C", "<hostRoot>", "."
	n := len(tarArgs)
	require.GreaterOrEqual(n, 3)
	require.Equal("-C", tarArgs[n-3])
	require.Equal("/home/x/repo", tarArgs[n-2])
	require.Equal(".", tarArgs[n-1])
}
