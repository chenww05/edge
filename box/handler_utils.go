package box

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/example/minibox/camera/thermal_1"
	"github.com/example/minibox/utils"
)

func (h *handler) downloadS3(url string) (string, string) {
	timestamp := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	storagePath := h.device.GetConfig().GetDataStoreDir()
	filePath := fmt.Sprintf("%s/%s", storagePath, timestamp)
	out, _ := os.Create(filePath)
	defer out.Close()

	resp, _ := http.Get(url)
	defer resp.Body.Close()

	_, _ = io.Copy(out, resp.Body)
	return filePath, timestamp
}

func (h *handler) convertVideoFormat(filePath string) (string, string) {
	timestamp := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	storagePath := h.device.GetConfig().GetDataStoreDir()
	newFilePath := fmt.Sprintf("%s/%s", storagePath, timestamp)

	params := []string{
		filePath,
		newFilePath,
	}
	cmd := exec.Command("hisiavi", params...)
	_ = cmd.Run()

	return newFilePath, timestamp
}

func (h *handler) md5Sum(filePath string) string {
	buf, err := ioutil.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", md5.Sum(buf))
}

func (h *handler) processBackgroundSettings(settings *thermal_1.BackgroundSettings) {
	localAddr := utils.GetLocalIp()
	prefixUrl := fmt.Sprintf("%s:8081/api/static/", localAddr)

	if settings.QuestionnaireDetailImgEn != "" {
		filePath, fileName := h.downloadS3(settings.QuestionnaireDetailImgEn)
		settings.QuestionnaireDetailImgEnMd5 = h.md5Sum(filePath)
		settings.QuestionnaireDetailImgEn = fmt.Sprintf("%s%s", prefixUrl, fileName)
	}

	if settings.QuestionnaireDetailImgSp != "" {
		filePath, fileName := h.downloadS3(settings.QuestionnaireDetailImgSp)
		settings.QuestionnaireDetailImgSpMd5 = h.md5Sum(filePath)
		settings.QuestionnaireDetailImgSp = fmt.Sprintf("%s%s", prefixUrl, fileName)
	}
	if settings.Splash != "" {
		filePath, fileName := h.downloadS3(settings.Splash)
		splashNamePre := strings.Split(settings.SplashName, ".")[0]
		fileFormat := strings.Split(settings.SplashName, ".")[1]
		if fileFormat == "mp4" {
			filePath, fileName = h.convertVideoFormat(filePath)
			settings.SplashName = fmt.Sprintf("%s.avi", splashNamePre)
		}
		settings.SplashMd5 = h.md5Sum(filePath)
		settings.Splash = fmt.Sprintf("%s%s", prefixUrl, fileName)
	}
}
