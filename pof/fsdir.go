package pof

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
)

// FsDir is the type of a fsdir.
type FsDir string

// mkdir creates a directory with the required parameters.
func mkdir(dir string) error {
	fi, err := os.Stat(dir)

	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(dir, 0755)

			if err != nil {
				return fmt.Errorf("error while trying to mkdir %s: %w", dir, err)
			}

			fi, err = os.Stat(dir)

			if err != nil {
				return fmt.Errorf("error while trying to stat created %s: %w", dir, err)
			}
		} else {
			return fmt.Errorf("error while trying to stat %s: %w", dir, err)
		}
	}

	if !fi.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", dir)
	}

	return nil
}

// New creates a new fsdir.
func NewDir(dir string) (FsDir, error) {
	p, err := filepath.Abs(dir)

	if err != nil {
		return FsDir(p), err
	}

	err = mkdir(p)

	if err != nil {
		return FsDir(p), err
	}

	return FsDir(p), nil
}

func (t FsDir) Path(ps ...string) string {
	if len(ps) == 0 {
		return string(t)
	}

	return path.Join(append([]string{string(t)}, ps...)...)
}

// Get reads the file under the path ps and unmarshals its JSON contents into
// x.
func (t FsDir) Get(x interface{}, ps ...string) error {
	p := t.Path(ps...)
	err := mkdir(path.Dir(p))

	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(p)

	if err != nil {
		return fmt.Errorf("fsdir.Get read: %w", err)
	}

	err = json.Unmarshal(b, x)

	if err != nil {
		return fmt.Errorf("error while trying to unmarshal %s: %w", p, err)
	}

	return nil
}

// Set marshals the x value into JSON and writes it to the the path ps.
func (t FsDir) Set(x interface{}, ps ...string) error {
	p := t.Path(ps...)
	err := mkdir(path.Dir(p))

	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(x, "", "    ")

	if err != nil {
		return err
	}

	err = ioutil.WriteFile(p, b, 0644)

	if err != nil {
		return err
	}

	return nil
}

// Del deletes the file or directory under a given path.
func (t FsDir) Del(ps ...string) error {
	p := t.Path(ps...)
	return os.RemoveAll(p)
}

// Chmod changes the permissions of the file or directory under a given path.
func (t FsDir) Chmod(mode os.FileMode, ps ...string) error {
	p := t.Path(ps...)
	return os.Chmod(p, mode)
}

// Home returns the fsdir of the home directory of this component.
func Home() FsDir {
	exe, err := os.Executable()

	if err != nil {
		log.Fatal(err)
	}

	fp, err := filepath.EvalSymlinks(exe)

	if err != nil {
		log.Fatal(err)
	}

	fm, err := NewDir(path.Dir(fp))

	if err != nil {
		log.Fatal(err)
	}

	return fm
}
