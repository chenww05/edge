package sunell

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/turingvideo/minibox/apis/structs"
	"github.com/turingvideo/minibox/box"
	"github.com/turingvideo/minibox/camera/base"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/configs"
	"github.com/turingvideo/minibox/db"
	"github.com/turingvideo/minibox/utils"
)

func (c *SunellAPI) parseBody(ctx *gin.Context) (map[string]string, bool, error) {
	m := make(map[string]string, 0)
	reader := bufio.NewReader(ctx.Request.Body)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, false, err
		}
		if strings.Contains(line, "Heart Message") {
			// heartbeat
			return nil, true, nil
		}

		line = strings.Replace(line, "\r\n", "", 1)
		words := strings.SplitN(line, "=", 2)
		if len(words) != 2 {
			continue
		}
		key, val := words[0], words[1]
		m[key] = val
	}
	return m, false, nil
}

func (c *SunellAPI) createSunellEvent(data map[string]string) *SunellEvent {
	var event SunellEvent
	for key, val := range data {
		switch key {
		case DeviceID:
			event.DeviceID = val
		case TargetId:
			event.TargetID = val
		case AlarmType:
			event.Type = val
		case AlarmTime:
			event.TimeStamp, _ = strconv.Atoi(val)
			// TODO
			// use box now time at the present, maybe change event.TimeStamp to time.Time in future
			event.Time = time.Now().UTC()
		case AIPictureData:
			event.ImgBase64 = val

		//	face
		case FaceX:
			event.FaceX, _ = strconv.Atoi(val)
		case FaceY:
			event.FaceY, _ = strconv.Atoi(val)
		case FaceWidth:
			event.FaceWidth, _ = strconv.Atoi(val)
		case FaceHeight:
			event.FaceHeight, _ = strconv.Atoi(val)

			//	car
		case CarColor:
			event.CarColor, _ = strconv.Atoi(val)
		case CarMode:
			event.CarMode, _ = strconv.Atoi(val)
		}
	}

	event.ImgFormat = DefaultImgFormat
	return &event
}

func (c *SunellAPI) uploadEventToCloud(event *SunellEvent, cameraID int, eventType string, now, eventTime time.Time, meta *structs.MetaScanData) (string, error) {
	faceScan := &db.FaceScan{
		ImgBase64: event.ImgBase64,
		Format:    event.ImgFormat,
		Rect: db.FaceRect{
			X:      event.FaceX,
			Y:      event.FaceY,
			Width:  event.FaceWidth,
			Height: event.FaceHeight,
		},
	}

	f, err := c.uploadFileToS3(cameraID, faceScan)
	if err != nil {
		return "", err
	}

	return c.Box.UploadAICameraEvent(cameraID, f, now, now.Add(time.Second*1), eventTime, event.Type, meta)
}

func (c *SunellAPI) uploadFileToS3(cameraID int, face *db.FaceScan) (*utils.S3File, error) {
	filename, img, err := utils.SaveImage(c.Box.GetConfig().GetDataStoreDir(), face.ImgBase64)
	if err != nil {
		return nil, err
	}
	face.Rect = db.FaceRect{
		Height: img.Bounds().Dy(),
		Width:  img.Bounds().Dx(),
	}

	c.Logger.Debug().Str("filename", filename).Msg("Saved temp image file")
	defer func() {
		if err := os.Remove(filename); err != nil {
			c.Logger.Warn().Err(err).Str("filename", filename).Msg("unable to delete temp image file")
		}
	}()
	return c.Box.UploadS3ByTokenName(cameraID, filename, face.Rect.Height, face.Rect.Width, "jpg", box.TokenNameCameraEvent)
}

func (c *SunellAPI) saveEventToDB(sunellEvent *SunellEvent, cameraID int, eventType string, now time.Time) (uint, error) {
	picInfo := db.FacePicture{
		Data:   []byte(sunellEvent.ImgBase64),
		Height: sunellEvent.FaceHeight,
		Width:  sunellEvent.FaceWidth,
		X:      sunellEvent.FaceX,
		Y:      sunellEvent.FaceY,
		Format: sunellEvent.ImgFormat,
	}

	c.Logger.Info().Object("face_pic", &picInfo).Msg("saving picture to db")
	if err := c.DB.GetDBInstance().Create(&picInfo).Error; err != nil {
		return 0, err
	}

	event := &db.Event{
		CameraSN:    sunellEvent.DeviceID,
		CameraID:    int64(cameraID),
		StartedAt:   sunellEvent.Time,
		EndedAt:     sunellEvent.Time.Add(time.Second * 1),
		CreatedAt:   now,
		UpdatedAt:   now,
		Type:        eventType,
		PictureID:   picInfo.ID,
		PictureHash: fmt.Sprintf("%x", md5.Sum([]byte(sunellEvent.ImgBase64))),
	}
	b, _ := json.Marshal(sunellEvent)
	event.Data = string(b)

	err := c.DB.CreateEvent(event)
	if err != nil {
		c.Logger.Error().Err(err).Msg("failed to save event to db")
		return 0, err
	}
	return event.ID, nil
}

func (c *SunellAPI) handleEventVideo(remoteID string, camID int, startTime, endTime int64) (string, *utils.S3File, error) {
	// record video first
	baseCam, err := c.Box.GetCamera(camID)
	if err != nil {
		c.Logger.Error().Err(err).Msgf("camera %d not found, aborting", camID)
		return "", nil, err
	}
	cam, ok := baseCam.(base.AICamera)
	if !ok {
		c.Logger.Error().Msgf("camera %d not found, aborting", camID)
		return "", nil, err
	}
	videoName := fmt.Sprintf("%s.mp4", remoteID)
	videoPath, width, height, err := cam.RecordVideo("", startTime, endTime, videoName, false, configs.NormalDownloadSpeed)
	if err != nil {
		return "", nil, err
	}

	var s3File *utils.S3File
	var finalErr error
	for i := 0; i < 3; i++ {
		// upload video to s3
		finalErr = nil
		c.Logger.Debug().Msgf("sunell try upload video to s3, camera_id: %d, videopath: %s, time: %d", baseCam.GetID(), videoPath, i+1)
		s3File, err = c.Box.UploadS3ByTokenName(baseCam.GetID(), videoPath, height, width, "mp4", box.TokenNameCameraEvent)
		if err != nil {
			finalErr = fmt.Errorf("video file upload s3 error: %v", err)
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}

		// upload s3Info to cloud
		var videos []cloud.MediaVideo
		videos = append(videos, cloud.MediaVideo{
			File: cloud.File{
				Meta: cloud.Meta{
					FileSize:    s3File.FileSize,
					Size:        []int{s3File.Height, s3File.Width},
					ContentType: "video/" + s3File.Format,
				},
				Key:    s3File.Key,
				Bucket: s3File.Bucket,
			},
			StartedAt: time.Unix(startTime, 0).Format(utils.CloudTimeLayout),
			EndedAt:   time.Unix(endTime, 0).Format(utils.CloudTimeLayout),
		})
		c.Logger.Debug().Msgf("sunell try upload media to cloud, camera_id: %d, time: %d", baseCam.GetID(), i+1)
		err = c.Box.UploadEventMedia(remoteID, &cloud.Media{
			Videos: &videos,
		})

		if err != nil {
			finalErr = fmt.Errorf("video file upload failed: %v", err)
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}
		break
	}
	// do clean when success
	if len(videoPath) > 0 && finalErr == nil {
		if err := os.Remove(videoPath); err != nil {
			c.Logger.Warn().Err(err).Str("filename", videoPath).Msg("unable to delete temp image file")
		}
	}
	return videoPath, s3File, finalErr
}

func (c *SunellAPI) HandleUploadEvent(data map[string]string) error {
	var eventType string
	event := c.createSunellEvent(data)

	baseCam, err := c.Box.GetCameraBySN(event.DeviceID)
	if err != nil {
		c.Logger.Error().Err(err).Msgf("camera %s not found, aborting", event.DeviceID)
		return err
	}

	baseCam.Heartbeat(baseCam.GetIP(), "")
	cfg := c.Box.GetConfig()

	cameraID := baseCam.GetID()

	saveEvent := cfg.GetEventSavedHours() > 0
	uploadCloud := cameraID > 0 && !cfg.GetDisableCloud()
	uploadVideo := baseCam.GetUploadVideoEnabled()

	// do upload data here by different event type TODO other alarm type
	if alarmType, ok := data[AlarmType]; ok {
		switch alarmType {
		case AlarmTypeVehicle:
			eventType = cloud.Car
		case AlarmTypeFace:
			eventType = cloud.FaceTracking
		case AlarmTypeBody:
			eventType = cloud.Intrude
		default:
			return fmt.Errorf("[Error Request] Alarm type %s is not supported", alarmType)
		}
	} else {
		return errors.New("[Error Request] Alarm type not found in request body")
	}

	now := time.Now().UTC()
	var eventID uint
	ok := false
	if saveEvent {
		eventID, err = c.saveEventToDB(event, cameraID, eventType, now)
		if err == nil {
			ok = true
		}
	}

	meta := &structs.MetaScanData{}

	if uploadCloud {
		remoteID, err := c.uploadEventToCloud(event, cameraID, eventType, now, now, meta) // todo (nilicheng) change event time to event timestamp
		if err != nil {
			c.Logger.Error().Err(err).Msg("failed to upload event to cloud")
		}

		var cloudErr cloud.Err
		if ok && remoteID != "" {
			err = c.DB.UpdateEvent(eventID, remoteID, int64(cameraID))
			if err != nil {
				c.Logger.Error().Err(err).Uint("event_id", eventID).Str("remote_id", remoteID).Msg("failed to update event")
			}
			//	try to upload video here
			if uploadVideo {
				alarmTime, err := strconv.ParseInt(data[AlarmTime], 10, 64)
				if err != nil {
					c.Logger.Error().Err(err).Msg("[Error Request] cannot parse request body")
					return err
				}
				startTime := alarmTime - cfg.GetSecBeforeEvent()
				endTime := startTime + cfg.GetVideoClipDuration()

				videoPath, s3File, err := c.handleEventVideo(remoteID, cameraID, startTime, endTime)
				if err != nil {
					c.Logger.Error().Err(err).Msg("[Error Request] cannot parse request body")
					var s3FileStr string
					if s3File != nil {
						buff, _ := json.Marshal(s3File)
						s3FileStr = string(buff)
					}
					err = c.DB.UpdateEventVideo(eventID, videoPath, s3FileStr, true)
					c.Logger.Error().Err(err).Msg("failed to update event video info")
					return err
				}
			}
		} else if remoteID == "" && !saveEvent && !errors.As(err, &cloudErr) {
			_, err := c.saveEventToDB(event, cameraID, eventType, now)
			if err != nil {
				c.Logger.Error().Err(err).Msg("failed to save event to db, LOST EVENT")
				return err
			}
		}
	} else if cameraID == 0 {
		c.Logger.Error().Msg("Camera has no id")
		if !saveEvent {
			_, err := c.saveEventToDB(event, cameraID, eventType, now)
			if err != nil {
				c.Logger.Error().Err(err).Msg("failed to save event to db, LOST EVENT")
			}
		}
	}
	return nil
}

func (c *SunellAPI) getCameraBySN(sn string) (base.AICamera, error) {
	camera, err := c.Box.GetCameraBySN(sn)
	if err != nil {
		return nil, err
	}

	cameraBrand := camera.GetBrand()
	if cameraBrand != utils.Sunell {
		err := errors.New(CameraBrandMissMatchErr)
		return nil, err
	}

	return camera.(base.AICamera), nil
}

func (c *SunellAPI) syncTimeZone(camera base.AICamera) {
	// Convert Location Timezone to Offset Timezone then set timezone to camera,
	// for invalid Location, camera timezone will be set to UTC
	timezoneText := c.Box.GetConfig().GetTimeZone()

	err := camera.SetTimeZone(timezoneText)
	if err != nil {
		c.Logger.Err(err).Send()
	}
}
