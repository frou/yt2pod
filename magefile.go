//+build mage

package main

import (
	"fmt"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func DetectDirtyGitRepo() error {
	status, err := sh.Output("git", "status", "--porcelain")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("git repository is dirty")
	}
	return nil
}

func BuildWithBakedVersion() error {
	mg.Deps(DetectDirtyGitRepo)

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

// ------------------------------------------------------------

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
