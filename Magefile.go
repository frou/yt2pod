//+build mage

package main

import (
	"fmt"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// @todo #0 Get rid of mage and write a bash script ./task that takes an argument

var Default = StampedBuild

func GitRepoIsClean() error {
	status, err := sh.Output("git", "status", "--porcelain")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("git repository is dirty")
	}
	return nil
}

func StampedBuild() error {
	mg.Deps(GitRepoIsClean)

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

	varsSetByLinker := map[string]string{
		"main.stampedBuildVersion": ourVersion,
		"main.stampedBuildTime":    time.Now().Format(time.RFC3339),
	}
	var linkerArgs string
	for name, value := range varsSetByLinker {
		linkerArgs += fmt.Sprintf(" -X %s=%s", name, value)
	}

	return sh.RunV("go", "build", "-ldflags", linkerArgs)
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
