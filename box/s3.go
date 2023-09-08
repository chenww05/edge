package box

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/utils"
)

func (b *baseBox) newUploadRequest(url, field, filename string, params map[string]string) (*http.Request, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for k, v := range params {
		_ = writer.WriteField(k, v)
	}

	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		return nil, err
	}
	if b.GetConfig().GetUploadConfig().EnableGateway {
		base64Encoded, err := utils.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		io.Copy(part, strings.NewReader(base64Encoded))
	} else {
		io.Copy(part, file)
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, err
}

type s3Response struct {
	XMLName  xml.Name `xml:"PostResponse"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

type lambdaResult struct {
	Location string `json:"Location"`
	Bucket   string `json:"Bucket"`
	Key      string `json:"Key"`
	ETag     string `json:"ETag"`
}

type gwResponse struct {
	ErrMsg string       `json:"err_msg"`
	Data   lambdaResult `json:"data"`
}

func isImageFile(filename string) bool {
	names := strings.Split(filename, ".")
	if len(names) > 2 {
		switch names[len(names)-1] {
		case "jpg", "gif", "png", "tiff":
			return true
		}
	}
	return false
}

func (b *baseBox) UploadS3ByTokenName(cameraId int, filename string, height, width int, format, tokenName string) (*utils.S3File, error) {
	f, err := os.Stat(filename)
	if err != nil {
		b.logger.Error().Msgf("stat file error: %s", err)
		return nil, err
	}
	contentType := mime.TypeByExtension("." + format)
	if len(contentType) == 0 {
		return nil, fmt.Errorf("invalid content type")
	}
	s3File, err := b.uploadMediaFileToS3(cameraId, filename, contentType, tokenName)
	if err != nil {
		b.logger.Error().Err(err).Msgf("upload to s3 error when token error")
		// upload error may token is fail,so del this token,get new token from cloud
		if !utils.IsNetworkError(err) {
			cacheKey, err := b.formatCacheKey(cameraId, tokenName)
			if err != nil {
				b.logger.Error().Err(err).Msgf("token error")
			} else {
				b.delTokenInCache(cacheKey)
			}
		}
		s3File, err = b.uploadMediaFileToS3(cameraId, filename, contentType, tokenName)
		if err != nil {
			b.logger.Error().Err(err).Msgf("retry 1 time error when token error")
			return nil, err
		}
	}

	s3File.FileSize = int(f.Size())
	s3File.Format = format
	s3File.Height = height
	s3File.Width = width
	return s3File, err
}

func (b *baseBox) uploadMediaFileToS3(cameraId int, filename, contentType, tokenName string) (*utils.S3File, error) {
	var token *cloud.Token
	var err error
	if cameraId <= 0 || tokenName == TokenNameCameraSnap {
		// Camera not created in cloud, if we get token by camera, it will be failed, we should get token by box here
		// This case only use for validate dvr or validate camera when we do add camera.
		// TokenNameCameraSnap must use org.SnapsExpire, so also do get token by box.
		token, err = b.GetTokenByBox(tokenName)
		if err != nil {
			return nil, err
		}
	} else {
		token, err = b.GetTokenByCamera(cameraId, tokenName)
		if err != nil {
			return nil, err
		}
	}

	ext := filepath.Ext(filename)
	if ext == "" {
		extArray := strings.Split(contentType, "/")
		if len(extArray) >= 2 {
			ext = "." + extArray[1]
		}
	}

	replacer := strings.NewReplacer(
		"{camera_id}", strconv.Itoa(cameraId),
		"{filename}", uuid.New().String()+ext)
	token.Fields.Key = replacer.Replace(token.Fields.Key)

	tokeFieldsBytes, _ := json.Marshal(token.Fields)
	var params map[string]string
	if err := json.Unmarshal(tokeFieldsBytes, &params); err != nil {
		return nil, err
	}
	params["Content-Type"] = contentType

	url := token.Url
	enableGW := b.GetConfig().GetUploadConfig().EnableGateway
	if enableGW {
		params["s3_url"] = token.Url
		url = b.GetConfig().GetUploadConfig().GatewayUploadUrl
	}
	b.logger.Debug().Msgf("upload file: %s to s3: %+v", filename, params["key"])
	req, err := b.newUploadRequest(url, "file", filename, params)
	if err != nil {
		return nil, err
	}

	res, err := b.s3httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	bucket, key := "", ""
	if enableGW {
		var body gwResponse
		if err = json.Unmarshal(content, &body); err != nil {
			return nil, err
		}
		if len(body.ErrMsg) != 0 {
			return nil, errors.New(body.ErrMsg)
		}
		bucket, key = body.Data.Bucket, body.Data.Key
	} else {
		var body s3Response
		if err = xml.Unmarshal(content, &body); err != nil {
			return nil, err
		}
		bucket, key = body.Bucket, body.Key
	}
	return &utils.S3File{
		Bucket: bucket,
		Key:    key,
	}, nil
}
