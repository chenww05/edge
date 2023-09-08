package box

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strconv"
	"time"

	goshawk "github.com/turingvideo/goshawk/uniview"
	"github.com/turingvideo/turing-common/model"
	"github.com/turingvideo/turing-common/websocket"

	"github.com/turingvideo/minibox/apis/structs"
	"github.com/turingvideo/minibox/camera/base"
	"github.com/turingvideo/minibox/camera/uniview"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/configs"
	"github.com/turingvideo/minibox/db"
	"github.com/turingvideo/minibox/utils"
)

const (
	EventTemperatureNormal   = "temperature_normal"
	EventTemperatureAbnormal = "temperature_abnormal"
	EventQuestionnaireFail   = "questionnaire_fail"

	defaultUploadEventsInterval = 60 * time.Second
	defaultOnceRetryCount       = 3

	defaultRefreshCameraSettingsInterval = 10 * time.Minute
	defaultRefreshIotDevicesInterval     = 10 * time.Minute
	defaultRefreshDeviceSettingsInterval = 5 * time.Minute

	eventRetryDelay = 5 * time.Minute
)

var ErrNoAPIClient = errors.New("API Client not initialized")

func (b *baseBox) UploadCameraEvent(cameraId int, file *utils.S3File, temperature, limit float64, meta *structs.MetaScanData, startAt time.Time) (string, error) {
	if b.apiClient == nil {
		return "", ErrNoAPIClient
	}

	var eventName string
	if meta != nil && meta.HasQuestionnaire && !meta.QuestionnaireResult {
		eventName = EventQuestionnaireFail
	} else if temperature > limit {
		eventName = EventTemperatureAbnormal
	} else {
		eventName = EventTemperatureNormal
	}

	val, ok := b.algoMap.Load(eventName)
	if !ok {
		err := fmt.Errorf("event type: %s is not found", eventName)
		b.logger.Error().Msgf("%s", err)
		return "", err
	}

	eventName, _ = val.(string)
	cameraEvent := cloud.CameraEvent{
		StartedAt: startAt.UTC(),
		CameraID:  cameraId,
		Algos:     []string{eventName},
		File:      file,
		Temperature: &cloud.TemperatureInfo{
			Temperature: temperature,
		},
		MetaScanData: meta,
	}

	event, err := b.apiClient.UploadCameraEvent(&cameraEvent)
	if event == nil {
		return "", err
	}

	return event.ID, err
}

func (b *baseBox) UploadPplEvent(cameraID int, meta *structs.MetaScanData, startAt time.Time, endedAt time.Time, eventType string) (string, error) {
	if b.apiClient == nil {
		return "", ErrNoAPIClient
	}
	val, ok := b.algoMap.Load(eventType)
	if !ok {
		err := fmt.Errorf("event type: %s is not found", eventType)
		b.logger.Error().Msgf("%s", err)
		return "", err
	}

	eventType, _ = val.(string)
	pplEvent := cloud.CameraEvent{
		StartedAt:    startAt,
		EndedAt:      endedAt,
		CameraID:     cameraID,
		Algos:        []string{eventType},
		MetaScanData: meta,
		IPCTime:      startAt,
	}
	event, err := b.apiClient.UploadPplEvent(&pplEvent)
	if event == nil {
		return "", err
	}
	return event.ID, err
}

func (b *baseBox) UploadAICameraEvent(cameraID int, file *utils.S3File, startAt, endAt, timestamp time.Time,
	eventType string, meta *structs.MetaScanData) (string, error) {
	if b.apiClient == nil {
		return "", errors.New("API Client not initialized")
	}

	val, ok := b.algoMap.Load(eventType)
	if !ok {
		err := fmt.Errorf("event type: %s is not found", eventType)
		b.logger.Error().Msgf("%s", err)
		return "", err
	}

	eventType, _ = val.(string)
	// cloud serial MetaScanData Temperature required, take a default value here
	cameraEvent := cloud.CameraEvent{
		StartedAt:    startAt.UTC(),
		EndedAt:      endAt.UTC(),
		IPCTime:      timestamp,
		CameraID:     cameraID,
		Algos:        []string{eventType},
		File:         file,
		MetaScanData: meta,
	}

	event, err := b.apiClient.UploadAICameraEvent(&cameraEvent)
	if event == nil {
		return "", err
	}

	return event.ID, err
}

func (b *baseBox) UploadEventMedia(eventID string, media *cloud.Media) error {
	if b.apiClient == nil {
		return errors.New("API Client not initialized")
	}
	return b.apiClient.UploadEventMedia(eventID, media)
}

func (b *baseBox) UploadCameraSnapshot(req *cloud.CamSnapShotReq) error {
	if b.apiClient == nil {
		return errors.New("API Client not initialized")
	}
	return b.apiClient.UploadCameraSnapshot(req)
}

func (b *baseBox) panicDisconnect() {
	if r := recover(); r != nil {
		b.logger.Error().Stack().Interface("error", r).Msg("Panic in cloud goroutine")

		b.DisconnectCloud()
	}
}

func (b *baseBox) uploadBoxTimezone(ctx context.Context) {
	defer b.panicDisconnect()

	heartBeatSecs := b.GetConfig().GetHeartBeatSecs() // 5s
	ticker := time.NewTicker(time.Duration(heartBeatSecs) * time.Second)
	defer ticker.Stop()
	oldStatusM := cloud.BoxState{}
	last := time.Now()

	for {
		select {
		case <-ctx.Done():
			b.logger.Info().Msg("Stopping upload box state")
			return
		case <-ticker.C:
			statusM := cloud.BoxState{Timezone: b.GetConfig().GetTimeZone()}

			curr := time.Now()
			// Need a 60 seconds heartbeat in case on change update failed.
			refreshMin := time.Duration(b.GetConfig().GetCameraRefreshMins())
			expired := last.Add(refreshMin * time.Minute).Before(curr)

			if b.apiClient != nil && (!reflect.DeepEqual(oldStatusM, statusM) || expired) {
				err := b.apiClient.UploadBoxTimezone(statusM, b.boxInfo.ID)
				if err != nil {
					b.logger.Error().Msgf("uploadBoxTimezone error: %s", err)
				} else {
					b.logger.Info().Interface("old_status", oldStatusM).Interface("new_status", statusM).
						Msg("box timezone uploaded to cloud")
					oldStatusM = statusM
					last = curr
				}
			}
		}
	}

}

func (b *baseBox) uploadCameraState(ctx context.Context) {
	defer b.panicDisconnect()

	heartBeatSecs := b.GetConfig().GetHeartBeatSecs() // 5s
	ticker := time.NewTicker(time.Duration(heartBeatSecs) * time.Second)
	defer ticker.Stop()
	oldStatusM := make(map[int]cloud.CameraState)
	last := time.Now()

	for {
		select {
		case <-ctx.Done():
			b.logger.Info().Msg("Stopping upload camera state")
			return
		case <-ticker.C:
			statusM := b.camGroup.GetAllCameraStatus()
			// A box with no cameras is a valid one
			if len(statusM) == 0 {
				continue
			}

			for _, camStatus := range statusM {
				if camStatus.State == model.CameraStateOffline {
					// This is to trigger the migration of the association relationship
					// when one camera is unplugged to the other NVR
					baseCam, _ := b.camGroup.GetCamera(camStatus.CameraID)
					_, ok := baseCam.(base.AICamera)
					if ok {
						b.RefreshDevices()
					}
					break
				}
			}

			curr := time.Now()
			// Need a 60 seconds heartbeat in case on change update failed.
			refreshMin := time.Duration(b.GetConfig().GetCameraRefreshMins())
			expired := last.Add(refreshMin * time.Minute).Before(curr)

			if b.apiClient != nil && (!reflect.DeepEqual(oldStatusM, statusM) || expired) {
				err := b.apiClient.UploadCameraState(statusM)
				if err != nil {
					b.logger.Error().Msgf("uploadCameraState error: %s", err)
				} else {
					b.logger.Info().Interface("old_status", oldStatusM).Interface("new_status", statusM).Msg("camera state uploaded to cloud")
					oldStatusM = statusM
					last = curr
				}
			}
		}
	}
}

func (b *baseBox) CloudClient() cloud.Client {
	return b.apiClient
}

func (b *baseBox) WsClient() websocket.Client {
	return b.wsClient
}

func mapCameraBySN(cameras []*cloud.Camera) map[string]*cloud.Camera {
	out := make(map[string]*cloud.Camera)
	for _, camera := range cameras {
		out[camera.SN] = camera
	}

	return out
}

func (b *baseBox) updateEventsWithoutCameraID() error {
	camEvents, err := b.db.GetEventsWithoutCameraID()
	if err != nil {
		return err
	}

	l := len(camEvents)
	if l > 0 {
		b.logger.Info().Msgf("Found %d events without cameraID", l)
		cameras, err := b.apiClient.GetCameras()
		if err != nil {
			return err
		}

		cameraMap := mapCameraBySN(cameras)

		for _, event := range camEvents {
			if camera, ok := cameraMap[event.CameraSN]; ok {
				if err := b.db.UpdateEvent(event.ID, event.RemoteID, int64(camera.ID)); err != nil {
					b.logger.Error().Err(err).Uint("event_id", event.ID).Msg("UpdateEvent error")
				} else {
					b.logger.Info().Int("camera_id", camera.ID).
						Uint("event_id", event.ID).
						Msg("Updated camera ID for event")
				}
			}
		}
	}

	return nil
}

func (b *baseBox) uploadPicToS3(cfg configs.Config, event *db.Event, pic *db.FacePicture) (*utils.S3File, error) {
	if pic == nil {
		return nil, nil
	}

	filename, img, err := utils.SaveImage(cfg.GetDataStoreDir(), string(pic.Data))
	if err != nil {
		b.logger.Error().Uint("event_id", event.ID).Msgf("SaveImage error: %s", err)
		return nil, err
	}
	pic.Height = img.Bounds().Dy()
	pic.Width = img.Bounds().Dx()

	b.logger.Debug().Str("filename", filename).Msg("saved temp image file")
	defer func() {
		if err := os.Remove(filename); err != nil {
			b.logger.Error().Str("filename", filename).Err(err).Msg("could not delete temp image file")
		}
	}()

	f, err := b.UploadS3ByTokenName(int(event.CameraID), filename, pic.Height, pic.Width, "jpg", TokenNameCameraEvent)
	if err != nil {
		b.logger.Error().Uint("event_id", event.ID).Msgf("UploadS3ByTokenName error: %s", err)
		return f, err
	}
	return f, nil
}

func (b *baseBox) retryUploadThermalEvent(event *db.Event, cfg configs.Config) (string, error) {
	// thermal
	var pic *db.FacePicture
	var err error
	if event.PictureID != 0 {
		pic, err = b.db.GetFacePicture(event.PictureID)
		if err != nil {
			b.logger.Error().Uint("event_id", event.ID).Msgf("GetFacePicture error: %s", err)
			return "-1", err
		}
	}

	// Check event.data first to avoid create files again.
	var meta structs.MetaScanData
	s, err := strconv.Unquote(event.Data)
	if err != nil {
		b.logger.Error().Str("data", event.Data).Err(err).Msg("Unquote error")
		return "-1", err
	}

	if err = json.Unmarshal([]byte(s), &meta); err != nil {
		b.logger.Error().Uint("event_id", event.ID).Msgf("UploadCameraEvent error: %s", err)
		return "-1", err
	}

	meta.RetryCount = event.RetryCount

	f, err := b.uploadPicToS3(cfg, event, pic)
	if err != nil {
		return "-1", err
	}
	return b.UploadCameraEvent(int(event.CameraID), f, meta.Temperature, meta.Abnormal, &meta, event.CreatedAt)
}

func (b *baseBox) retryUploadAICameraEvent(event *db.Event, cfg configs.Config) (string, error) {
	var pic *db.FacePicture
	var err error
	if event.PictureID != 0 {
		pic, err = b.db.GetFacePicture(event.PictureID)
		if err != nil {
			b.logger.Error().Uint("event_id", event.ID).Msgf("GetFacePicture error: %s", err)
			return "-1", err
		}
	}

	f, err := b.uploadPicToS3(cfg, event, pic)
	if err != nil {
		return "-1", err
	}

	// Check event.data first to avoid create files again.
	var meta structs.MetaScanData
	s, err := strconv.Unquote(event.Data)
	if err != nil {
		b.logger.Error().Str("data", event.Data).Err(err).Msg("Unquote error")
		return "-1", err
	}

	if err = json.Unmarshal([]byte(s), &meta); err != nil {
		b.logger.Error().Uint("event_id", event.ID).Msgf("retryUploadAICameraEvent error: %s", err)
		return "-1", err
	}

	return b.UploadAICameraEvent(int(event.CameraID), f, event.StartedAt, event.EndedAt, event.IPCTime, event.Type, &meta)
}

func (b *baseBox) handleRetryUploadEvents() {
	b.logger.Info().Msg("Start retry upload events")
	// Update events without camera ID from cloud
	if err := b.updateEventsWithoutCameraID(); err != nil {
		b.logger.Error().Err(err).Msg("Update Events Camera ID error")
		return
	}

	cfg := b.GetConfig()

	events, err := b.db.GetRetryEvents()
	if err != nil {
		b.logger.Error().Msgf("GetRetryEvents error: %s", err)
		return
	}
	b.logger.Info().Msgf("Found %d events to retry upload", len(events))

	for _, event := range events {
		timeDiff := time.Now().Sub(event.CreatedAt)

		if timeDiff < eventRetryDelay {
			continue
		}

		hourDiff := int(timeDiff.Hours())
		if hourDiff >= cfg.GetEventSavedHours() {
			// Mark cloud_error as true as it exceeds the max saved hour
			if err := b.db.EventStopRetry(event.ID); err != nil {
				b.logger.Error().Err(err).Uint("event_id", event.ID).Msg("Mark stop retry error")
			}
			continue
		}

		// Make sure event retry cfg.GetEventRetryCount() times in 1 hour
		if event.RetryCount > uint(hourDiff+1)*cfg.GetEventRetryCount() {
			b.logger.Error().Err(err).Msgf("event video upload %d reach to max retry.", event.CameraID)
			continue
		}

		// local cameras and remote cameras are all same in thermal and vision, we don't need to retry the event which
		// camera_id not found in local cameras (remote cameras)
		if _, err := b.GetCamera(int(event.CameraID)); err != nil {
			b.logger.Error().Err(err).Msgf("camera %d not found, aborting", event.CameraID)
			continue
		}

		var remoteID string
		var err error
		if event.Type == db.EventQuestionnaireFail ||
			event.Type == db.EventTemperatureAbnormal ||
			event.Type == db.EventTemperatureNormal {
			remoteID, err = b.retryUploadThermalEvent(&event, cfg)
			if err != nil {
				b.logger.Error().Err(err).Msg("failed to upload event to cloud!")
			}
		} else if event.Type == cloud.FaceTracking ||
			event.Type == cloud.Car ||
			event.Type == cloud.Intrude ||
			event.Type == cloud.LicensePlate ||
			event.Type == cloud.MotorCycleIntrude ||
			event.Type == cloud.MotorCycleEnter ||
			event.Type == cloud.MotionStart {
			remoteID, err = b.retryUploadAICameraEvent(&event, cfg)
			if err != nil {
				b.logger.Error().Err(err).Msg("failed to upload ai camera event to cloud!")
			}
		}
		if remoteID == "-1" || remoteID == "" {
			b.logger.Error().Err(err).Msg("failed to re-upload event to cloud!")
			if err := b.db.IncrementRetryEvent(event.ID); err != nil {
				b.logger.Error().Err(err).Uint("event_id", event.ID).Msg("Increment retry error")
			}
			continue
		}

		// update event
		err = b.db.UpdateEvent(event.ID, remoteID, event.CameraID)
		if err != nil {
			b.logger.Error().Uint("event_id", event.ID).Msgf("UpdateEvent error: %s", err)
			continue
		}

		buff, _ := json.Marshal(event)
		b.logger.Info().Str("remote_id", remoteID).RawJSON("event", buff).Msg("Uploaded event in background")
	}
}

func (b *baseBox) handleRetryUploadEventVideos() {
	b.logger.Info().Msg("Start retry upload event video")
	events, err := b.db.GetRetryEventVideos()
	if err != nil {
		b.logger.Error().Msgf("GetEventVideoRetryList error: %s", err)
		return
	}
	cfg := b.GetConfig()
	for _, event := range events {
		timeDiff := time.Now().Sub(event.CreatedAt)

		hourDiff := int(timeDiff.Hours())
		// Video retry must start after event upload succeed.
		// If event upload succeed in last retry (last retry in 5 hours), video hove no chance to retry upload when limit 5 hours
		// So we make video retry hours = event save hours + 1
		if hourDiff >= cfg.GetEventSavedHours()+db.ExtraHoursForVideoRetry {
			// Del local video
			if len(event.VideoPath) > 0 {
				if err := os.Remove(event.VideoPath); err != nil {
					b.logger.Warn().Err(err).Str("remoteID", event.RemoteID).Str("filename", event.VideoPath).Msg("unable to delete temp image file")
				}
			}
			// Stop upload video
			if err = b.db.UpdateEventVideo(event.ID, "", "", false); err != nil {
				b.logger.Error().Str("remoteID", event.RemoteID).Err(err).Msgf("UpdateEventVideo %d", event.CameraID)
			}
			continue
		}

		if event.VideoRetryCount+event.RetryCount > uint(hourDiff+1)*cfg.GetEventRetryCount() {
			b.logger.Error().Err(err).Msgf("event %d reach to max retry.", event.CameraID)
			continue
		}

		baseCam, err := b.GetCamera(int(event.CameraID))
		if err != nil {
			// if camera not found don't do retryCount++
			b.logger.Error().Err(err).Msgf("camera %d not found, aborting", event.CameraID)
			continue
		}
		cam, ok := baseCam.(base.AICamera)
		if !ok {
			// if camera not found don't do retryCount++
			b.logger.Error().Str("remoteID", event.RemoteID).Msgf("camera %d not found, aborting", event.CameraID)
			continue
		}

		b.logger.Debug().Str("remoteID", event.RemoteID).Msgf("RetryUpdateEventVideo event %v", event.ID)
		var finalErr error
		secBeforeEvent := b.GetConfig().GetSecBeforeEvent()
		startTime := event.StartedAt.Unix() - secBeforeEvent
		endTime := startTime + cfg.GetVideoClipDuration()
		videoPath := event.VideoPath
		// record when videoPath null
		if videoPath == "" {
			videoName := fmt.Sprintf("%s.mp4", event.RemoteID)
			resolution := utils.Normal
			if cam.GetManufacturer() != utils.TuringUniview {
				resolution = utils.HD
			}
			videoPath, _, _, err = cam.RecordVideo(string(resolution), startTime, endTime, videoName, false, configs.NormalDownloadSpeed)
			if err != nil {
				finalErr = err
				b.logger.Error().Err(err).Str("remoteID", event.RemoteID).Msg("RecordVideo Error")
			}
		}

		var s3File *utils.S3File
		if finalErr == nil {
			//  try to upload s3
			if err := json.Unmarshal([]byte(event.VideoS3File), &s3File); err != nil {
				b.logger.Warn().Str("remoteID", event.RemoteID).Msgf("Token Marshal Error %v", err)
			}
			if s3File == nil {
				var s3Err error
				for i := 0; i < defaultOnceRetryCount; i++ {
					s3Err = nil
					s3File, err = b.UploadS3ByTokenName(int(event.CameraID), videoPath, 0, 0, "mp4", TokenNameCameraEvent)
					if err != nil {
						s3Err = fmt.Errorf("video file upload s3 error: %v", err)
						time.Sleep(time.Second * time.Duration(i+1))
						b.logger.Error().Str("remoteID", event.RemoteID).Err(s3Err).Msgf("upload media file to s3 error %d", event.CameraID)
						continue
					}
					break
				}
				if s3Err != nil {
					finalErr = s3Err
				}
			}
		}
		if finalErr == nil {
			//  try upload to cloud
			var cloundErr error
			for i := 0; i < defaultOnceRetryCount; i++ {
				// upload s3Info to cloud
				cloundErr = nil
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
				b.logger.Debug().Msgf("try upload media: %s to event: %s, camera_id: %d, time: %d", s3File.Key, event.RemoteID, event.CameraID, i+1)
				err = b.UploadEventMedia(event.RemoteID, &cloud.Media{
					Videos: &videos,
				})
				if err != nil {
					time.Sleep(time.Second * time.Duration(i+1))
					b.logger.Error().Str("remoteID", event.RemoteID).Msgf("upload event media to cloud error %d", event.CameraID)
					cloundErr = err
					continue
				}
				break
			}
			if cloundErr != nil {
				finalErr = cloundErr
			}
		}

		if finalErr == nil {
			// upload success
			b.logger.Debug().Str("remoteID", event.RemoteID).Msgf("RetryUpdateEventVideo success event %d", event.ID)
			if err = b.db.UpdateEventVideo(event.ID, "", "", false); err != nil {
				b.logger.Error().Str("remoteID", event.RemoteID).Err(err).Msgf("UpdateEventVideo %d", event.CameraID)
			}

			if len(videoPath) > 0 {
				if err := os.Remove(videoPath); err != nil {
					b.logger.Warn().Err(err).Str("remoteID", event.RemoteID).Str("filename", videoPath).Msg("unable to delete temp image file")
				}
			}
		} else {
			// upload failed
			b.logger.Error().Str("remoteID", event.RemoteID).Err(finalErr).Msgf("RetryUpdateEventVideo fail event %d", event.ID)
			// retry++
			if err := b.db.IncrementRetryEventVideo(event.ID); err != nil {
				b.logger.Error().Err(err).Uint("event_id", event.ID).Msg("Increment retry error")
			}
			// update video resource in db
			var s3FileStr string
			if s3File != nil {
				buff, _ := json.Marshal(s3File)
				s3FileStr = string(buff)
			}
			_ = b.db.UpdateEventVideo(event.ID, videoPath, s3FileStr, true)
		}
	}
}

func (b *baseBox) retryUploadEvents(ctx context.Context) {
	defer b.panicDisconnect()

	ticker := time.NewTicker(defaultUploadEventsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.logger.Info().Msg("Stopping upload events")
			return
		case <-ticker.C:
			// random in 5s
			time.Sleep(time.Duration(rand.Intn(5000) * 1000 * 1000))
			b.handleRetryUploadEvents()
			b.handleRetryUploadEventVideos()
		}
	}
}

func (b *baseBox) updateSnapshotRule() {
	cams := b.GetCamGroup().AllCameras()
	// enable nvr snapshot
	nvrSystemSnapshotRuleEnable := make(map[string]bool, 0)
	for _, cam := range cams {
		if aiCam, ok := cam.(base.AICamera); ok {
			if cloud.HasEventType(aiCam.GetID(), cloud.MotionStart) {
				nvrSystemSnapshotRuleEnable[aiCam.GetNvrSN()] = true
			}
		}
	}
	for nvrSn, enable := range nvrSystemSnapshotRuleEnable {
		nc, err := b.GetNVRManager().GetNVRClientBySN(nvrSn)
		if err != nil {
			continue
		}
		if err := nc.PutSystemSnapshotRule(enable); err != nil {
			b.logger.Error().Msgf("put system snapshot rule error %s", err)
		}
	}
}

func (b *baseBox) updateLocalCameraSettings() {
	cams := b.GetCamGroup().AllCameras()
	var ids = make([]int, 0, len(cams))

	for _, cam := range cams {
		if aiCam, ok := cam.(base.AICamera); ok {
			if aiCam.GetOnline() && b.nvrManager.IsCameraPlugged(aiCam) {
				ids = append(ids, aiCam.GetID())
			}
		}
	}

	// Only AI camera is ok
	if len(ids) == 0 {
		cloud.ClearCameraSettings()
		return
	}
	settings, err := b.apiClient.GetCameraSettings(ids)
	if err != nil {
		b.logger.Error().Err(err).Msg("failed to update local camera settings")
		return
	}
	// Get camera settings from cloud and save them into memory.
	cloud.SaveCameraSettings(settings)
	b.updateSnapshotRule()
}

func (b *baseBox) fetchCameraSettingsFromNvr(c base.AICamera) *cloud.CameraSettings {
	nvrCli, err := b.GetNVRManager().GetNVRClientBySN(c.GetNvrSN())
	if err != nil {
		return nil
	}

	channel := c.GetChannel()

	// Video stream info
	localStreamSettings, err := nvrCli.GetChannelStreamDetailInfo(channel)
	if err != nil || localStreamSettings == nil {
		b.logger.Err(err).Msgf("refresh device settings: get channel stream detail info")
		return nil
	}

	// Media video capabilities
	localVideoCapabilities, err := nvrCli.GetChannelMediaVideoCapabilities(channel)
	if err != nil || localVideoCapabilities == nil {
		b.logger.Err(err).Msgf("refresh device settings: get channel media video capabilities")
		return nil
	}

	univCam, ok := c.(*uniview.BaseUniviewCamera)
	if ok {
		univCam.SetLocalStreamSettings(localStreamSettings)
	}
	// Audio switch info
	localAudioStatus, err := nvrCli.GetAudioDecodeStatuses(channel)
	if err != nil || localAudioStatus == nil {
		b.logger.Err(err).Msgf("refresh device settings: get channel audio settings")
		return nil
	}

	// Osd settings
	localOSDSettings, err := nvrCli.GetOSDSettings(channel)
	if err != nil || localOSDSettings == nil {
		b.logger.Err(err).Msgf("refresh device settings: get channel osd settings")
		return nil
	}

	univCam, ok = c.(*uniview.BaseUniviewCamera)
	if ok {
		univCam.SetCachedOSDSettings(localOSDSettings)
	}

	// Osd capabilities
	localOSDCapabilities, err := nvrCli.GetOSDCapabilities(channel)
	if err != nil || localOSDCapabilities == nil {
		b.logger.Err(err).Msgf("refresh device settings: get channel osd capabilities")
		return nil
	}

	var localAudioInput *goshawk.AudioInputCfg
	localAudioInput, err = nvrCli.GetChannelMediaAudioInput(channel)
	if err == goshawk.ErrorInvalidLAPI {
		// If this API is not supported by this NVR, just go through
	} else if err != nil || localAudioInput == nil {
		b.logger.Err(err).Msgf("refresh device settings: get channel audio input")
		return nil
	}
	var localRecordRecordSchedule *goshawk.RecordScheduleInfo
	localRecordRecordSchedule, err = nvrCli.GetChannelStorageScheduleRecord(channel)
	if err == goshawk.ErrorInvalidLAPI {
		// If this API is not supported by this NVR, just go through
	} else if err != nil || localRecordRecordSchedule == nil {
		b.logger.Err(err).Msgf("refresh device settings: get channel record schedule")
		return nil
	}

	localCameraSettings := &cloud.CameraSettings{
		CamID:             c.GetID(),
		CamSN:             c.GetSN(),
		CloudEventTypes:   []string{},                // api in cloud will ignore this param, so just give []string{}
		CloudEventMeta:    &cloud.CloudEventMeta{},   // api in cloud will ignore this param, so just give &cloud.CloudEventMeta{}
		CloudStorageMeta:  &cloud.CloudStorageMeta{}, // api in cloud will ignore this param, so just give &cloud.CloudEventMeta{}
		StreamSettings:    localStreamSettings.VideoStreamInfos,
		VideoCapabilities: localVideoCapabilities,
		AudioSettings:     localAudioStatus,
		OSDSettings:       localOSDSettings,
		OSDCapabilities:   localOSDCapabilities,
		AudioInput:        localAudioInput,
		RecordSchedule:    localRecordRecordSchedule,
	}

	if isCameraSettingsChanged(localCameraSettings, cloud.GetCameraSettingsByID(c.GetID())) {
		b.updateCameraOSDTextAreas() // We should only update this by NVR data, the only source of truth
		return localCameraSettings
	} else {
		return nil
	}
}

// Get local ipc settings, and compare with local memory settings.
// If not same, update cloud camera settings.
func (b *baseBox) updateRemoteCameraSettings() {
	cameras := b.GetCamGroup().AllCameras()
	pluggedCams := []base.AICamera{}
	for _, cam := range cameras {
		if aiCam, ok := cam.(base.AICamera); ok {
			if aiCam.GetOnline() && b.nvrManager.IsCameraPlugged(aiCam) {
				pluggedCams = append(pluggedCams, aiCam)
			}
		}
	}

	// Upload to cloud
	var settings []*cloud.CameraSettings
	for _, c := range pluggedCams {
		setting := b.fetchCameraSettingsFromNvr(c)
		if setting != nil {
			settings = append(settings, setting)
		}
	}

	if len(settings) > 0 {
		_ = b.CloudClient().UploadCameraSettings(settings)
	}
}

func (b *baseBox) syncCameraSettings(ctx context.Context) {
	defer b.panicDisconnect()

	ticker1 := time.NewTicker(defaultRefreshCameraSettingsInterval)
	defer ticker1.Stop()
	ticker2 := time.NewTicker(defaultRefreshDeviceSettingsInterval)
	defer ticker2.Stop()
	for {
		select {
		case <-ctx.Done():
			b.logger.Info().Msg("stopping update camera settings")
			return
		case <-ticker1.C:
			b.updateLocalCameraSettings()
		case <-ticker2.C:
			b.updateRemoteCameraSettings()
		}
	}
}

func isSameVideoStream(src, dest []goshawk.VideoStreamInfo) bool {
	// ID = 0 & MainStreamType = 1 as type3
	srcStreamMap := make(map[uint32]goshawk.VideoStreamInfo, 1)
	destStreamMap := make(map[uint32]goshawk.VideoStreamInfo, 1)
	for _, stream := range src {
		if stream.ID == 0 && stream.MainStreamType == 1 {
			srcStreamMap[3] = stream
		} else {
			srcStreamMap[stream.ID] = stream
		}
	}
	for _, stream := range dest {
		if stream.ID == 0 && stream.MainStreamType == 1 {
			destStreamMap[3] = stream
		} else {
			destStreamMap[stream.ID] = stream
		}
	}
	if len(srcStreamMap) != len(destStreamMap) {
		return false
	}
	for streamId, srcStream := range srcStreamMap {
		destStream, ok := destStreamMap[streamId]
		if !ok {
			return false
		}
		if srcStream != destStream {
			return false
		}
	}
	return true
}

func isSameAudioSettings(src, dest []goshawk.AudioStatus) bool {
	srcMap := make(map[int]goshawk.AudioStatus, 1)
	destMap := make(map[int]goshawk.AudioStatus, 1)
	for _, audio := range src {
		srcMap[audio.StreamID] = audio
	}
	for _, audio := range dest {
		destMap[audio.StreamID] = audio
	}
	if len(srcMap) != len(destMap) {
		return false
	}
	for streamId, srcAudio := range srcMap {
		destAudio, ok := destMap[streamId]
		if !ok {
			return false
		}
		if srcAudio != destAudio {
			return false
		}
	}
	return true
}

func isSameMediaVideoCapabilities(src, dest *goshawk.MediaVideoCapabilities) bool {
	if src == nil && dest == nil {
		return true
	} else if src == nil || dest == nil {
		return false
	}

	if src.IsSupportCfg != dest.IsSupportCfg ||
		src.IsSupportSmoothLevel != dest.IsSupportSmoothLevel ||
		src.IsSupportImageFormat != dest.IsSupportImageFormat ||
		src.IsSupportScrambled != dest.IsSupportScrambled ||
		src.EncodeFormatNum != dest.EncodeFormatNum ||
		src.MinIFrameInterval != dest.MinIFrameInterval ||
		src.MaxIFrameInterval != dest.MaxIFrameInterval ||
		src.StreamCapabilityNum != dest.StreamCapabilityNum ||
		src.VideoModeNum != dest.VideoModeNum ||
		src.GOPTypeNum != dest.GOPTypeNum {
		return false
	}

	sort.SliceStable(src.EncodeFormatList, func(i, j int) bool {
		return src.EncodeFormatList[i] < src.EncodeFormatList[j]
	})
	sort.SliceStable(dest.EncodeFormatList, func(i, j int) bool {
		return dest.EncodeFormatList[i] < dest.EncodeFormatList[j]
	})
	if len(src.EncodeFormatList) != len(dest.EncodeFormatList) {
		return false
	}
	for index, v := range src.EncodeFormatList {
		if v != dest.EncodeFormatList[index] {
			return false
		}
	}

	sort.SliceStable(src.VideoModeInfoList, func(i, j int) bool {
		if src.VideoModeInfoList[i].Resolution.Width == src.VideoModeInfoList[j].Resolution.Width {
			if src.VideoModeInfoList[i].Resolution.Height == src.VideoModeInfoList[j].Resolution.Height {
				return src.VideoModeInfoList[i].FrameRate > src.VideoModeInfoList[j].FrameRate
			} else {
				return src.VideoModeInfoList[i].Resolution.Height > src.VideoModeInfoList[j].Resolution.Height
			}
		} else {
			return src.VideoModeInfoList[i].Resolution.Width > src.VideoModeInfoList[j].Resolution.Width
		}
	})
	sort.SliceStable(dest.VideoModeInfoList, func(i, j int) bool {
		if dest.VideoModeInfoList[i].Resolution.Width == dest.VideoModeInfoList[j].Resolution.Width {
			if dest.VideoModeInfoList[i].Resolution.Height == dest.VideoModeInfoList[j].Resolution.Height {
				return dest.VideoModeInfoList[i].FrameRate > dest.VideoModeInfoList[j].FrameRate
			} else {
				return dest.VideoModeInfoList[i].Resolution.Height > dest.VideoModeInfoList[j].Resolution.Height
			}
		} else {
			return dest.VideoModeInfoList[i].Resolution.Width > dest.VideoModeInfoList[j].Resolution.Width
		}
	})
	if len(src.VideoModeInfoList) != len(dest.VideoModeInfoList) {
		return false
	}
	for index, sv := range src.VideoModeInfoList {
		dv := dest.VideoModeInfoList[index]
		if sv != dv {
			return false
		}
	}

	// StreamCapabilityList
	srcMap := make(map[int]goshawk.StreamCapability, 1)
	destMap := make(map[int]goshawk.StreamCapability, 1)
	for _, v := range src.StreamCapabilityList {
		srcMap[v.ID] = v
	}
	for _, v := range dest.StreamCapabilityList {
		destMap[v.ID] = v
	}
	if len(srcMap) != len(destMap) {
		return false
	}
	for streamId, sv := range srcMap {
		dv, ok := destMap[streamId]
		if !ok {
			return false
		}
		svJson, err := json.Marshal(sv)
		if err != nil {
			return false
		}
		dvJson, err := json.Marshal(dv)
		if err != nil {
			return false
		}
		if string(svJson) != string(dvJson) {
			return false
		}
	}
	return true
}

func isSameOSDSettings(src, dest *goshawk.OSDSetting) bool {
	if src == nil && dest == nil {
		return true
	} else if src == nil || dest == nil {
		return false
	}
	// check num
	if src.Num != dest.Num {
		return false
	}
	// check OSDContentStyle
	if src.ContentStyle != dest.ContentStyle {
		return false
	}
	// check ContentList
	if len(src.ContentList) != len(dest.ContentList) {
		return false
	}
	srcMap := make(map[int]goshawk.OSDContent, 1)
	destMap := make(map[int]goshawk.OSDContent, 1)
	for _, content := range src.ContentList {
		srcMap[content.ID] = content
	}
	for _, content := range dest.ContentList {
		destMap[content.ID] = content
	}
	for id, sv := range srcMap {
		dv, ok := destMap[id]
		if !ok {
			return false
		}
		if sv.ID != dv.ID {
			return false
		}
		if sv.Num != dv.Num {
			return false
		}
		if sv.Area != dv.Area {
			return false
		}
		if sv.Enabled != dv.Enabled {
			return false
		}
		// check ContentInfoList
		if len(sv.ContentInfoList) != len(dv.ContentInfoList) {
			return false
		}
		srcInfoMap := make(map[int]goshawk.OSDContentInfo, 1)
		destInfoMap := make(map[int]goshawk.OSDContentInfo, 1)
		for _, info := range sv.ContentInfoList {
			srcInfoMap[info.ContentType] = info
		}
		for _, info := range dv.ContentInfoList {
			destInfoMap[info.ContentType] = info
		}
		for contentType, s := range srcInfoMap {
			d, ok := destInfoMap[contentType]
			if !ok {
				return false
			}
			if s.ContentType != d.ContentType {
				return false
			}
			if s.Value != d.Value {
				return false
			}
		}
	}
	return true
}

func isSameOSDCapabilities(src, dest *goshawk.OSDCapabilities) bool {
	if src == nil && dest == nil {
		return true
	} else if src == nil || dest == nil {
		return false
	}
	if src.IsSupportCfg != dest.IsSupportCfg {
		return false
	}
	if src.SupportedOSDContentTypeNum != dest.SupportedOSDContentTypeNum {
		return false
	}
	if src.IsSupportFontSizeCfg != dest.IsSupportFontSizeCfg {
		return false
	}
	if src.IsSupportFontColorCfg != dest.IsSupportFontColorCfg {
		return false
	}
	if src.MaxAreaNum != dest.MaxAreaNum {
		return false
	}
	if src.MaxOSDNum != dest.MaxOSDNum {
		return false
	}
	if src.MaxPerAreaOSDNum != dest.MaxPerAreaOSDNum {
		return false
	}
	if src.SupportedTimeFormatNum != dest.SupportedTimeFormatNum {
		return false
	}
	if src.SupportedDateFormatNum != dest.SupportedDateFormatNum {
		return false
	}

	if len(src.SupportedOSDContentTypeList) != len(dest.SupportedOSDContentTypeList) {
		return false
	}
	sort.SliceStable(src.SupportedOSDContentTypeList, func(i, j int) bool {
		return src.SupportedOSDContentTypeList[i] < src.SupportedOSDContentTypeList[j]
	})
	sort.SliceStable(dest.SupportedOSDContentTypeList, func(i, j int) bool {
		return dest.SupportedOSDContentTypeList[i] < dest.SupportedOSDContentTypeList[j]
	})
	for index, v := range src.SupportedOSDContentTypeList {
		if v != dest.SupportedOSDContentTypeList[index] {
			return false
		}
	}

	if len(src.SupportedTimeFormatList) != len(dest.SupportedTimeFormatList) {
		return false
	}
	sort.SliceStable(src.SupportedTimeFormatList, func(i, j int) bool {
		return src.SupportedTimeFormatList[i] < src.SupportedTimeFormatList[j]
	})
	sort.SliceStable(dest.SupportedTimeFormatList, func(i, j int) bool {
		return dest.SupportedTimeFormatList[i] < dest.SupportedTimeFormatList[j]
	})
	for index, v := range src.SupportedTimeFormatList {
		if v != dest.SupportedTimeFormatList[index] {
			return false
		}
	}

	if len(src.SupportedDateFormatList) != len(dest.SupportedDateFormatList) {
		return false
	}
	sort.SliceStable(src.SupportedDateFormatList, func(i, j int) bool {
		return src.SupportedDateFormatList[i] < src.SupportedDateFormatList[j]
	})
	sort.SliceStable(dest.SupportedDateFormatList, func(i, j int) bool {
		return dest.SupportedDateFormatList[i] < dest.SupportedDateFormatList[j]
	})
	for index, v := range src.SupportedDateFormatList {
		if v != dest.SupportedDateFormatList[index] {
			return false
		}
	}
	return true
}

func isSameAudioInput(src, dest *goshawk.AudioInputCfg) bool {
	if src == nil && dest == nil {
		return true
	} else if src == nil || dest == nil {
		return false
	}
	srcJson, err := json.Marshal(src)
	if err != nil {
		return false
	}
	destJson, err := json.Marshal(dest)
	if err != nil {
		return false
	}
	return string(srcJson) == string(destJson)
}

func isSameRecordSchedule(src, dest *goshawk.RecordScheduleInfo) bool {
	if src == nil && dest == nil {
		return true
	} else if src == nil || dest == nil {
		return false
	}
	srcJson, err := json.Marshal(src)
	if err != nil {
		return false
	}
	destJson, err := json.Marshal(dest)
	if err != nil {
		return false
	}
	return string(srcJson) == string(destJson)
}

func isCameraSettingsChanged(local, remote *cloud.CameraSettings) bool {
	if local == nil {
		// local nil don't update the cloud data, just return false
		return false
	}
	if remote == nil {
		// remote nil means there is not data in cloud, just update local data to cloud
		return true
	}

	// video stream changed just return true
	if !isSameVideoStream(local.StreamSettings, remote.StreamSettings) {
		return true
	}

	// audio stream changed just return true
	if !isSameAudioSettings(local.AudioSettings, remote.AudioSettings) {
		return true
	}

	// video capabilities changed just return true
	if !isSameMediaVideoCapabilities(local.VideoCapabilities, remote.VideoCapabilities) {
		return true
	}

	// osd settings changed just return true
	if !isSameOSDSettings(local.OSDSettings, remote.OSDSettings) {
		return true
	}

	// osd capabilities changed just return true
	if !isSameOSDCapabilities(local.OSDCapabilities, remote.OSDCapabilities) {
		return true
	}

	// Audio input changed just return true
	if !isSameAudioInput(local.AudioInput, remote.AudioInput) {
		return true
	}

	// Record schedule changed just return true
	if !isSameRecordSchedule(local.RecordSchedule, remote.RecordSchedule) {
		return true
	}
	return false
}

func (b *baseBox) checkIotDevice(macIotDeviceMap map[string]*cloud.IotDevice) (miss bool) {
	reportInterval := b.config.GetArpReportInterval()
	reportTimes := int(b.config.GetArpReportTimes())
	for i := 0; i < reportTimes; i++ {
		miss = false
		time.Sleep(time.Duration(reportInterval) * time.Second)
		arpDeviceMap := b.arpSearcher.GetDevices()
		for mac, iotDevice := range macIotDeviceMap {
			if iotDevice.Updated {
				continue
			}
			if arpDevice, ok := arpDeviceMap[mac]; ok {
				iotDevice.IpAddress = arpDevice.IP
				iotDevice.Updated = true
			} else {
				miss = true
			}
		}
		if !miss {
			// Every device found, break the for loop
			break
		}
	}
	return
}

func (b *baseBox) updateIotDevices() {
	if !b.config.GetArpEnable() {
		b.logger.Error().Msg("arp not enable")
		return
	}
	iotDevices, err := b.apiClient.GetIotDevices()
	if err != nil {
		b.logger.Error().Err(err).Msg("init failed to get camera error")
		return
	}
	ips := []string{}
	macs := []string{}
	macIotDeviceMap := make(map[string]*cloud.IotDevice)
	for index := range iotDevices {
		ips = append(ips, iotDevices[index].IpAddress)
		macs = append(macs, iotDevices[index].MacAddress)
		macIotDeviceMap[iotDevices[index].MacAddress] = iotDevices[index]
	}

	if b.arpSearcher.CacheExpired() {
		b.arpSearcher.SearchDevices(ips, macs)
		if b.checkIotDevice(macIotDeviceMap) {
			b.arpSearcher.SearchAll()
			b.checkIotDevice(macIotDeviceMap)
		}
	} else {
		b.checkIotDevice(macIotDeviceMap)
	}
	now := time.Now().UTC()
	for _, iotDevice := range macIotDeviceMap {
		if iotDevice.Updated {
			iotDevice.State = utils.DeviceStatusOnline
		} else {
			if iotDevice.State == utils.DeviceStatusOnline {
				// State change from online to offline here
				go func(dev cloud.IotDevice) {
					atime := time.Now().Format(utils.CloudTimeLayout)
					err = b.apiClient.UploadAlarmInfo(&cloud.AlarmInfo{
						Source:      cloud.AlarmSourceBridge,
						BoxId:       b.GetBoxId(),
						IotDeviceID: dev.ID,
						StartedAt:   atime,
						EndedAt:     atime,
						Detection: cloud.Detection{
							Algos: cloud.AlarmTypeIotDeviceOffline,
						},
					})
					if err != nil {
						b.logger.Debug().Msgf("iot device offline alarm failed, mac: %s", dev.MacAddress)
					}
				}(*iotDevice)
			}
			iotDevice.State = utils.DeviceStatusOffline
			b.logger.Debug().Msgf("iot device offline, mac: %s", iotDevice.MacAddress)
		}
		iotDevice.LastSyncTime = now
	}
	err = b.apiClient.UploadIotDevices(iotDevices)
	if err != nil {
		b.logger.Error().Err(err).Msg("upload iot devices error")
	}
}

func (b *baseBox) syncIotDevices(ctx context.Context) {
	ticker := time.NewTicker(defaultRefreshIotDevicesInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			b.logger.Info().Msg("stopping sync iot devices")
			return
		case <-ticker.C:
			go b.updateIotDevices()
		}
	}
}

func (b *baseBox) NotifyCloudEventVideoClipUploadFailed(id int) error {
	event, err := b.db.GetEvent(id)
	if err != nil {
		return err
	}
	if event.RemoteID == "" {
		errMsg := fmt.Sprintf("event %d upload failed", id)
		b.logger.Error().Msg(errMsg)
		return errors.New(errMsg)
	}
	return b.CloudClient().NotifyCloudEventVideoClipUploadFailed(event.CameraID, event.RemoteID)
}

func (b *baseBox) updateCameraOSDTextAreas() {
	cameras := b.GetCamGroup().AllCameras()
	for _, c := range cameras {
		if c.GetBrand() == utils.Uniview {
			univCam, ok := c.(*uniview.BaseUniviewCamera)
			if ok {
				univCam.UpdateCameraOSDTextAreas()
			}
		}
	}
}
