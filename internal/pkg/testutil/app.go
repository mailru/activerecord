package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mailru/activerecord/internal/pkg/ds"
)

var TestAppInfo = *ds.NewAppInfo().
	WithBuildOS(runtime.GOOS).
	WithBuildTime(time.Now().String()).
	WithVersion("1.0").
	WithBuildCommit("nocommit")

type Tmps struct {
	dirs []string
}

func InitTmps() *Tmps {
	return &Tmps{
		dirs: []string{},
	}
}

func (tmp *Tmps) AddTempDir(basepath ...string) (string, error) {
	rootTmpDir := os.TempDir()
	if len(basepath) > 0 {
		rootTmpDir = basepath[0]
	}

	newTempDir, err := os.MkdirTemp(rootTmpDir, "argen_testdir*")
	if err != nil {
		return "", fmt.Errorf("can't create temp dir for test: %s", err)
	}

	tmp.dirs = append(tmp.dirs, newTempDir)

	return newTempDir, nil
}

const (
	EmptyDstDir uint32 = 1 << iota
	NonExistsDstDir
	NonExistsSrcDir
)

func (tmp *Tmps) CreateDirs(flags uint32) (string, string, error) {
	projectDir, err := tmp.AddTempDir()
	if err != nil {
		return "", "", fmt.Errorf("can't create root test dir: %w", err)
	}

	srcDir := filepath.Join(projectDir, "src")
	if flags&NonExistsSrcDir != NonExistsSrcDir {
		if err := os.MkdirAll(srcDir, 0750); err != nil {
			return "", "", fmt.Errorf("can't create temp src dir: %w", err)
		}
	}

	dstDir := ""

	switch {
	case flags&NonExistsDstDir == NonExistsDstDir:
		dstDir = filepath.Join(projectDir, "nonexistsdst")
	case flags&EmptyDstDir == EmptyDstDir:
		dstDir = filepath.Join(projectDir, "dst")
		if err := os.MkdirAll(dstDir, 0750); err != nil {
			return "", "", fmt.Errorf("can't create temp dst dir: %w", err)
		}

		if err := os.WriteFile(filepath.Join(dstDir, ".argen"), []byte("test argen special file"), 0600); err != nil {
			return "", "", fmt.Errorf("can't create special file into dst")
		}
	}

	return srcDir, dstDir, nil
}

func (tmp *Tmps) Defer() {
	for _, dir := range tmp.dirs {
		os.RemoveAll(dir)
	}
}

func GetPathToSrc() string {
	_, filename, _, _ := runtime.Caller(0)
	filenameSplit := strings.Split(filename, string(filepath.Separator))

	return string(filepath.Separator) + filepath.Join(filenameSplit[:len(filenameSplit)-4]...)
}
