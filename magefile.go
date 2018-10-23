//+build mage

package main

import (
	"fmt"

	"github.com/magefile/mage/sh"
)

func BuildWithBakedVersion() error {
	dirty, err := gitRepositoryIsDirty()
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("git repository is dirty")
	}

	var ourVersion string
	tag, err := gitHeadTag()
	if err != nil {
		ourVersion, err = gitHeadSHA(true)
		if err != nil {
			return err
		}
	} else {
		ourVersion = tag
	}

	return sh.RunV("go", "build", "-ldflags", fmt.Sprintf("-X main.yt2podVersion=%s", ourVersion))
}

func gitRepositoryIsDirty() (bool, error) {
	status, err := sh.Output("git", "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return status != "", nil
}

func gitHeadTag() (string, error) {
	return sh.Output("git", "describe", "--tags", "--exact-match", "HEAD")
}

func gitHeadSHA(short bool) (string, error) {
	gitArgs := []string{"rev-parse"}
	if short {
		gitArgs = append(gitArgs, "--short")
	}
	gitArgs = append(gitArgs, "HEAD")
	return sh.Output("git", gitArgs...)
}
