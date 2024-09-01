package ffmpeg

import (
	"fmt"
	"os/exec"
)

func ResizeImage(source string, width int, height int, destination string) bool {
	result := exec.Command("ffmpeg", "-y", "-i", source, "-loop", "0", "-vf", "scale="+fmt.Sprint(width)+":"+fmt.Sprint(height), destination)
	err := result.Run()
	if err != nil {
		println("FFMPEG error: ", err.Error())
	}
	return err == nil
}

func ConvertImage(source string, destination string) bool {
	result := exec.Command("ffmpeg", "-y", "-i", source, "-loop", "0", destination)
	err := result.Run()
	if err != nil {
		println("FFMPEG error: ", err.Error())
	}
	return err == nil
}
