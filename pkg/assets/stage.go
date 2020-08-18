package assets

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Mirantis/mke/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// return true if currently running executable is older than given filepath
func ExecutableIsOlder(filepath string) bool {
	ex, err := os.Executable()
	if err != nil {
		return false
	}
	exinfo, err := os.Stat(ex)
	if err != nil {
		return false
	}
	pathinfo, err := os.Stat(filepath)
	if err != nil {
		return false
	}
	return exinfo.ModTime().Unix() < pathinfo.ModTime().Unix()
}

// StagedBinPath returns the path of the staged bin or the name without path if it does not exist
func StagedBinPath(dataDir, name string) string {
	p := filepath.Join(dataDir, "bin", name)
	if util.FileExists(p) {
		return p
	}
	return name
}

// Stage ...
func Stage(dataDir, name, group string) error {
	p := filepath.Join(dataDir, name)

	if ExecutableIsOlder(p) {
		logrus.Debug("Re-use existing file:", p)
		return nil
	}

	content, err := Asset(name)
	if err != nil || content == nil {
		return err
	}
	logrus.Debug("Writing static file: ", p)
	err = os.MkdirAll(filepath.Dir(p), 0750)
	if err != nil {
		return errors.Wrapf(err, "failed to create dir %s", filepath.Dir(p))
	}

	os.Remove(p)
	if err := ioutil.WriteFile(p, content, 0640); err != nil {
		return errors.Wrapf(err, "failed to write to %s", name)
	}
	if err := os.Chmod(p, 0550); err != nil {
		return errors.Wrapf(err, "failed to chmod %s", name)
	}

	gid, _ := util.GetGid(group)
	if gid != 0 {
		paths := []string{dataDir, filepath.Dir(p), p}
		for _, path := range paths {
			logrus.Debugf("setting group ownership for %s to %d", path, gid)
			err = os.Chown(path, -1, gid)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
