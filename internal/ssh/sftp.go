package ssh

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/pkg/sftp"
)

// withSFTP opens an SFTP subsystem and runs fn against it.
func (c *Client) withSFTP(fn func(*sftp.Client) error) error {
	sc, err := sftp.NewClient(c.ssh)
	if err != nil {
		return fmt.Errorf("open sftp: %w", err)
	}
	defer sc.Close()
	return fn(sc)
}

// MkdirAll creates a remote directory tree.
func (c *Client) MkdirAll(dir string) error {
	return c.withSFTP(func(sc *sftp.Client) error { return sc.MkdirAll(dir) })
}

// WriteFile writes data to remotePath with the given mode (parent dirs created).
func (c *Client) WriteFile(remotePath string, data []byte, mode os.FileMode) error {
	return c.withSFTP(func(sc *sftp.Client) error {
		if err := sc.MkdirAll(path.Dir(remotePath)); err != nil {
			return err
		}
		f, err := sc.Create(remotePath)
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		return sc.Chmod(remotePath, mode)
	})
}

// ReadFile reads a remote file's contents.
func (c *Client) ReadFile(remotePath string) ([]byte, error) {
	var out []byte
	err := c.withSFTP(func(sc *sftp.Client) error {
		f, err := sc.Open(remotePath)
		if err != nil {
			return err
		}
		defer f.Close()
		b, err := io.ReadAll(f)
		out = b
		return err
	})
	return out, err
}

// UploadFile copies a local file to remotePath with mode, atomically: it writes
// to remotePath+".new" then renames, avoiding "text file busy" on a running
// binary.
func (c *Client) UploadFile(localPath, remotePath string, mode os.FileMode) error {
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()
	return c.withSFTP(func(sc *sftp.Client) error {
		if err := sc.MkdirAll(path.Dir(remotePath)); err != nil {
			return err
		}
		tmp := remotePath + ".new"
		dst, err := sc.Create(tmp)
		if err != nil {
			return err
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			return err
		}
		if err := dst.Close(); err != nil {
			return err
		}
		if err := sc.Chmod(tmp, mode); err != nil {
			return err
		}
		// Rename over an existing file requires POSIX rename semantics.
		_ = sc.Remove(remotePath)
		return sc.Rename(tmp, remotePath)
	})
}

// Exists reports whether a remote path exists.
func (c *Client) Exists(remotePath string) (bool, error) {
	var exists bool
	err := c.withSFTP(func(sc *sftp.Client) error {
		_, statErr := sc.Stat(remotePath)
		if statErr == nil {
			exists = true
			return nil
		}
		if os.IsNotExist(statErr) {
			return nil
		}
		return statErr
	})
	return exists, err
}
