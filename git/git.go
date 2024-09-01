package git

import (
	"emlserver/structs"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func Clone(url string, destination string) error {
	if !strings.HasPrefix(strings.ToLower(url), "https://") {
		return errors.New("git url is not https.")
	}

	err := validateInput(url)
	if err != nil {
		return err
	}
	_, err = runCommand(*exec.Command("git", "clone", url, destination))
	return err
}

func Update(path string) error {
	_, err := runCommand(*exec.Command("git", "-C", path, "fetch"))
	if err != nil {
		return err
	}

	_, err = runCommand(*exec.Command("git", "-C", path, "pull"))
	return err
}

func GetRemoteBranches(url string) ([]string, error) {
	err := validateInput(url)
	if err != nil {
		return nil, err
	}
	res, err := runCommand(*exec.Command("git", "ls-remote", "--refs", url))
	resString := string(res)
	if err != nil {
		return nil, err
	}
	var branches []string
	split := strings.Split(resString, "\n")
	for _, element := range split {

		if len(element) < 41 {
			continue
		}

		gitBranch := element[41:]
		if strings.Contains(gitBranch, "heads") {
			branchName := strings.Split(gitBranch, "/")[2]
			branches = append(branches, branchName)
		}
	}

	return branches, nil
}

func GetCommits(path string) ([]structs.ResponseCommit, error) {
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return []structs.ResponseCommit{}, err
	}
	commits, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return []structs.ResponseCommit{}, err
	}

	var commitFormatted []structs.ResponseCommit

	commits.ForEach(func(commit *object.Commit) error {
		commitFormatted = append(commitFormatted, structs.ResponseCommit{
			Author:        commit.Author.Name + " (" + commit.Author.Email + ")",
			CommitContent: commit.Message,
			Hash:          commit.ID().String(),
			Timestamp:     fmt.Sprint(commit.Author.When.UnixMilli()),
		})
		return nil
	})
	return commitFormatted, nil
}

func runCommand(cmd exec.Cmd) (string, error) {
	res, err := cmd.Output()
	return string(res), err
}

func validateInput(input string) error {
	if strings.Contains(input, " ") {
		return errors.New("User input contains spaces.")
	}
	return nil
}
