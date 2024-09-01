package archive

import (
	"os/exec"
)

func Package(srcFolder string, dst string, extraArgs []string) error {
	cmd := exec.Command("tar", "-czf", dst, "-C", srcFolder, "--exclude-vcs")
	cmd.Args = append(cmd.Args, extraArgs...)
	cmd.Args = append(cmd.Args, ".")
	err := cmd.Run()
	return err
}

func Extract(src string, dst string) error {
	err := exec.Command("tar", "-xzf", src, "-C", dst).Run()
	return err
}
