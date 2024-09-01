package image

import (
	"emlserver/ffmpeg"
	"fmt"
)

func GenerateProfilePicture(input string, id string) {
	ffmpeg.ResizeImage(input, 256, 256, fmt.Sprint("static/pfp/", id, ".webp"))
}

func GenerateModIcon(input string, id string) {
	ffmpeg.ResizeImage(input, 256, 256, fmt.Sprint("static/modimg/", id, ".webp"))
}
