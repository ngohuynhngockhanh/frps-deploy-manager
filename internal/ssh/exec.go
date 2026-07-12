package ssh

import (
	"bytes"
	"fmt"
	"strings"

	xssh "golang.org/x/crypto/ssh"
)

// Result is the outcome of a remote command.
type Result struct {
	Stdout string
	Exit   int
	Stderr string
}

// Run executes cmd and captures stdout/stderr/exit code. A non-zero exit is
// reported via Result.Exit (not as an error); transport errors are returned.
func (c *Client) Run(cmd string) (Result, error) {
	sess, err := c.ssh.NewSession()
	if err != nil {
		return Result{}, err
	}
	defer sess.Close()

	var out, errb bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &errb
	runErr := sess.Run(cmd)

	res := Result{Stdout: out.String(), Stderr: errb.String()}
	if runErr == nil {
		return res, nil
	}
	var ee *xssh.ExitError
	if errorsAs(runErr, &ee) {
		res.Exit = ee.ExitStatus()
		return res, nil
	}
	return res, runErr
}

// MustRun runs cmd and returns an error if the exit code is non-zero.
func (c *Client) MustRun(cmd string) (string, error) {
	r, err := c.Run(cmd)
	if err != nil {
		return "", err
	}
	if r.Exit != 0 {
		return r.Stdout, fmt.Errorf("command failed (exit %d): %s: %s", r.Exit, cmd, strings.TrimSpace(r.Stderr))
	}
	return r.Stdout, nil
}

// Out runs cmd and returns trimmed stdout (ignoring exit code).
func (c *Client) Out(cmd string) string {
	r, _ := c.Run(cmd)
	return strings.TrimSpace(r.Stdout)
}

// errorsAs is a tiny local wrapper to avoid importing errors just for As.
func errorsAs(err error, target **xssh.ExitError) bool {
	if e, ok := err.(*xssh.ExitError); ok {
		*target = e
		return true
	}
	return false
}
