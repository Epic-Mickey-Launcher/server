package captcha

import (
	"bytes"
	"emlserver/config"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func CreateCaptcha() string {
	buffer, err := os.ReadFile("captcha.html")
	if err != nil {
		return ""
	}

	page := string(buffer)
	page = strings.ReplaceAll(page, "{SITEKEY}", config.LoadedConfig["CAPTCHA_SITEKEY"])

	return page
}

func ValidateCaptcha(token string) bool {
	body := bytes.NewBuffer([]byte(fmt.Sprintf("secret=%s&response=%s", config.LoadedConfig["CAPTCHA_SECRET"], token)))
	response, err := http.Post("https://challenges.cloudflare.com/turnstile/v0/siteverify", "application/x-www-form-urlencoded", body)
	if err != nil {
		println("failed to get response from cloudflare siteverify")
		return false
	}

	buffer, err := io.ReadAll(response.Body)
	if err != nil {
		return false
	}

	var result map[string]any

	println(string(buffer))

	json.Unmarshal(buffer, &result)

	res, ok := result["success"]

	if !ok {
		return false
	}

	return res.(bool)
}
