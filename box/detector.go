package box

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/example/minibox/utils"
)

const ObjectDetectRetryTimes = 3

func (b *baseBox) objectDetect(body []byte) (error, []utils.DetectObjects) {
	resp, err := http.Post(b.GetConfig().GetDetectorCfg().DetectorEndpoint,
		"application/json",
		bytes.NewBuffer(body))
	if err != nil {
		return err, []utils.DetectObjects{}
	}

	defer func() {
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	ret, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err, []utils.DetectObjects{}
	}
	detectionResp := utils.ObjectDetectionResp{}
	if err := json.Unmarshal(ret, &detectionResp); err != nil {
		b.logger.Error().Msgf("Unmarshal ObjectDetectionResp error: %s", err)
		return err, []utils.DetectObjects{}
	}
	return nil, detectionResp.Data.Objects
}

func (b *baseBox) ObjectDetect(imgData string) (error, []utils.DetectObjects) {
	dataDir := b.GetConfig().GetDataStoreDir()
	dataDir = path.Clean(dataDir)
	filePath, _, err := utils.SaveImage(dataDir, imgData)
	if err != nil {
		b.logger.Error().Err(err)
		return err, nil
	}
	defer func() {
		if err := os.Remove(filePath); err != nil {
			b.logger.Warn().Err(err).Str("filename", filePath).Msg("unable to delete temp image file")
		}
	}()
	// relative path
	f := strings.Split(filePath, dataDir)[1]
	f = strings.TrimPrefix(f, "/")
	req := utils.ObjectDetectionReq{
		ImagePath: f,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err, nil
	}

	var finalErr error
	var objects []utils.DetectObjects
	for i := 0; i < ObjectDetectRetryTimes; i++ {
		finalErr, objects = b.objectDetect(body)
		if finalErr == nil {
			break
		}
	}
	return finalErr, objects
}
