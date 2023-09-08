package uniview

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
	goshawk "github.com/example/goshawk/uniview"
	"github.com/example/minibox/configs"
	"github.com/example/turing-common/model"

	"github.com/example/minibox/apis/structs"
	"github.com/example/minibox/box"
	"github.com/example/minibox/camera/base"
	"github.com/example/minibox/camera/uniview"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/db"
	"github.com/example/minibox/utils"
)

func parseNVRSN(reference string) string {
	refs := strings.Split(reference, "/")
	if len(refs) < 2 {
		return ""
	}
	return refs[1]
}

func (u *UniviewAPI) getCamera(nvrSN string, channel uint32) (*uniview.BaseUniviewCamera, error) {
	cameras := u.Box.GetCamGroup().AllCameras()
	for _, cam := range cameras {
		if cam.GetBrand() == utils.Uniview {
			univCam, ok := cam.(*uniview.BaseUniviewCamera)
			if ok && univCam.GetNvrSN() == nvrSN && univCam.GetChannel() == channel && univCam.GetID() > 0 {
				if univCam.GetOnline() {
					return univCam, nil
				} else {
					u.Logger.Warn().Msgf("camera %d(%s) is not online, replaced cameras should be disconnected from cloud", univCam.ID, univCam.Name)
				}
			}
		}
	}
	return nil, fmt.Errorf("this camera is not belong to %s or not online", nvrSN)
}

func (u *UniviewAPI) notifyCloudEventVideoClipUploadFailed(remoteID string, eventID uint, videoUploaded, uploadVideoEnable bool) {
	startTime := time.Now()
	for len(remoteID) > 0 && !videoUploaded && uploadVideoEnable {
		if time.Now().Sub(startTime).Minutes() > 5 {
			break
		}
		if err := u.Box.NotifyCloudEventVideoClipUploadFailed(int(eventID)); err != nil {
			u.Logger.Error().Err(err).Msgf("failed to notify event video upload failed for event %d", eventID)
			time.Sleep(time.Second * 5)
		} else {
			break
		}
	}
}

func (u *UniviewAPI) processEvent(univCam *uniview.BaseUniviewCamera, eventType string, eventTime int64, imgBase64 string,
	meta *structs.MetaScanData, saveEvent, uploadCloud, uploadVideo bool, recvTime time.Time, videoDuration int64) (remoteID string) {

	remoteID, eventID, videoUploaded := u.innerProcessEvent(univCam, eventType, eventTime, imgBase64, meta, saveEvent, uploadCloud, uploadVideo, recvTime, videoDuration)
	go u.notifyCloudEventVideoClipUploadFailed(remoteID, eventID, videoUploaded, uploadVideo)

	return remoteID
}

func (u *UniviewAPI) innerProcessEvent(univCam *uniview.BaseUniviewCamera, eventType string, eventTime int64, imgBase64 string,
	meta *structs.MetaScanData, saveEvent, uploadCloud, uploadVideo bool, recvTime time.Time, videoDuration int64) (remoteID string, eventID uint, videoUploaded bool) {

	if !cloud.HasEventType(univCam.ID, eventType) {
		u.Logger.Warn().Msgf("you don't have license: %s on this camera: %d", eventType, univCam.ID)
		return
	}
	var err error
	var saved bool
	var videoPath string
	var s3File *utils.S3File

	// Try to convert coordinates in meta
	w, h, whErr := utils.ImageWidthHeight(imgBase64)
	if whErr != nil {
		u.Logger.Error().Err(whErr).Msgf("failed to calc image width height for camera %d, ipc time: %d", univCam.ID, eventTime)
	} else {
		u.convertCoordinates(w, h, meta)
	}

	// save event in local db
	if saveEvent {
		if eventID, err = u.saveEventToDB(univCam.GetSN(), univCam.GetID(), eventType, imgBase64, meta, eventTime, recvTime); err == nil {
			saved = true
		}
	}

	// upload event to cloud
	if uploadCloud {
		if remoteID, err = u.uploadEventToCloud(imgBase64, univCam.ID, eventType, recvTime, time.Unix(eventTime, 0), meta); err != nil || remoteID == "" {
			u.Logger.Error().Err(err).Msg("failed to upload event to cloud")
		}
	}

	// upload video to cloud
	if uploadVideo && len(remoteID) > 0 { // event upload success and enable upload video
		startTime := eventTime - u.Box.GetConfig().GetSecBeforeEvent()
		endTime := startTime + videoDuration
		// ensure the event end time is before now.
		duration := time.Unix(endTime, 0).Sub(time.Now())
		if duration > 0 {
			time.Sleep(duration)
		}
		if videoPath, s3File, err = u.handleEventVideo(remoteID, univCam.GetID(), startTime, endTime); err != nil {
			u.Logger.Error().Err(err).Msgf("camera %d handle event video error,remote id %s", univCam.GetID(), remoteID)
		} else {
			videoUploaded = true
		}
	}

	// update event in db
	if !saved {
		return
	}

	// upload event to cloud failed, videoPath s3File must null
	if len(remoteID) == 0 {
		// update videoFailed true
		if err = u.DB.UpdateEventVideo(eventID, "", "", uploadVideo); err != nil {
			u.Logger.Error().Str("eventType", eventType).Str("remoteID", remoteID).Err(err).
				Msgf("failed to update event video info %v+", err)
		}
		return
	}

	// upload event to cloud success, update remoteID firstly
	if err = u.DB.UpdateEvent(eventID, remoteID, int64(univCam.ID)); err != nil {
		u.Logger.Error().Err(err).Uint("eventID", eventID).
			Str("remoteID", remoteID).Msg("failed to update event")
		return
	}

	// update videoFailed
	if videoUploaded || !uploadVideo {
		// if video upload success, update videoFailed false and uploadVideo flag must be true
		// if we don't need to upload video, we can also mark videoFailed as false
		if err = u.DB.UpdateEventVideo(eventID, "", "", false); err != nil {
			u.Logger.Error().Str("eventType", eventType).Str("remoteID", remoteID).Err(err).
				Msgf("failed to update event video info %v+", err)
		}
		return
	}

	// if video upload failed, update videoFailed true
	var s3FileStr string
	if s3File != nil {
		buff, _ := json.Marshal(s3File)
		s3FileStr = string(buff)
	}
	if err = u.DB.UpdateEventVideo(eventID, videoPath, s3FileStr, true); err != nil {
		u.Logger.Error().Str("eventType", eventType).Str("remoteID", remoteID).Err(err).
			Msgf("failed to update event video info %v+", err)
	}

	return
}

func (u *UniviewAPI) parsePosition(position string) (rect structs.Rectangle, err error) {
	r, _ := regexp.Compile("([0-9]+),([0-9]+);([0-9]+),([0-9]+)")
	results := r.FindAllStringSubmatch(position, -1)
	if (len(results) != 1) || (len(results[0]) != 5) {
		err = fmt.Errorf("position data format error, the data is %s", position)
		return
	}

	coordinates := make([]int, 4)
	for index := 1; index < 5; index++ {
		coordinates[index-1], _ = strconv.Atoi(results[0][index])
	}

	rect = structs.Rectangle{
		X:      float64(coordinates[0]),
		Y:      float64(coordinates[1]),
		Width:  float64(coordinates[2] - coordinates[0]),
		Height: float64(coordinates[3] - coordinates[1]),
	}
	return
}

func (u *UniviewAPI) assembleEvent(univCam *uniview.BaseUniviewCamera, srcName, eventType string, timestamp int64,
	saveEvent, uploadCloud, uploadVideo bool,
	imageList []ImageInfo, objectInfoList []ObjectDetected, recvTime time.Time, videoDuration int64) {
	for index, picture := range imageList {
		objs := make([]structs.ObjectInfo, 0)
		for _, item := range objectInfoList {
			if int(item.GetLargePicIndex()-1) == index {
				rect, err := u.parsePosition(item.GetPosition())
				if err != nil {
					u.Logger.Error().Err(err).Send()
					continue
				}
				objInfo := structs.ObjectInfo{
					Confidence: int(item.GetConfidence()),
					BBox:       rect,
				}
				objs = append(objs, objInfo)
			}
		}
		if len(objs) > 0 {
			polygonInfos := u.getPolygonInfos(univCam, srcName, eventType)
			snapshotOSDTextAreas := u.getSnapshotOSDTextAreas(univCam)
			videoOSDTextAreas := u.getVideoOSDTextAreas(univCam)
			meta := &structs.MetaScanData{
				Objects:              objs,
				PolygonInfos:         polygonInfos,
				SnapshotOSDTextAreas: snapshotOSDTextAreas,
				VideoOSDTextAreas:    videoOSDTextAreas,
			}
			go u.processEvent(univCam, eventType, timestamp, picture.Data, meta, saveEvent, uploadCloud, uploadVideo, recvTime, videoDuration)
		}
	}
}

func (u *UniviewAPI) handlerLocalDetectEvent(univCam *uniview.BaseUniviewCamera, eventTime int64, imageInfoList []ImageInfo,
	saveEvent, uploadCloud, uploadVideo bool, recvTime time.Time, videoDuration int64) error {
	detectParams := univCam.GetDetectParams()
	// for onvif event
	if len(imageInfoList) == 0 {
		return fmt.Errorf("onvif event no image")
	}
	// always 1 img
	for _, img := range imageInfoList {
		err, objects := u.Box.ObjectDetect(img.Data)
		if err != nil {
			u.Logger.Error().Err(err)
			continue
		}

		var carObjs []structs.ObjectInfo
		var peopleObjs []structs.ObjectInfo
		var motorcycleObjs []structs.ObjectInfo
		for _, object := range objects {
			objInfo := structs.ObjectInfo{
				Confidence: int(object.Confidence * 100),
				BBox: structs.Rectangle{
					X:      float64(object.Xmin),
					Y:      float64(object.Ymin),
					Width:  float64(object.Xmax - object.Xmin),
					Height: float64(object.Ymax - object.Ymin),
				},
			}
			switch object.Label {
			case EventTruck, EventBoat, EventBus, EventTrain, EventCar:
				if object.Confidence < detectParams.CarThreshold {
					continue
				}
				carObjs = append(carObjs, objInfo)
			case EventPeople:
				if object.Confidence < detectParams.PersonThreshold {
					continue
				}
				peopleObjs = append(peopleObjs, objInfo)
			case EventMotorcycle:
				if object.Confidence < detectParams.MotorcycleThreshold {
					continue
				}
				motorcycleObjs = append(motorcycleObjs, objInfo)
			default:
			}
		}
		if len(carObjs) > 0 {
			meta := &structs.MetaScanData{Objects: carObjs}
			go u.processEvent(univCam, cloud.Car, eventTime, img.Data, meta, saveEvent, uploadCloud, uploadVideo, recvTime, videoDuration)
		}
		if len(peopleObjs) > 0 {
			meta := &structs.MetaScanData{Objects: peopleObjs}
			go u.processEvent(univCam, cloud.Intrude, eventTime, img.Data, meta, saveEvent, uploadCloud, uploadVideo, recvTime, videoDuration)
		}
		if len(motorcycleObjs) > 0 {
			meta := &structs.MetaScanData{Objects: motorcycleObjs}
			go u.processEvent(univCam, cloud.MotorCycleIntrude, eventTime, img.Data, meta, saveEvent, uploadCloud, uploadVideo, recvTime, videoDuration)
		}
	}
	return nil
}

func (u *UniviewAPI) handleStandardEvent(univCam *uniview.BaseUniviewCamera, notification EventNotification,
	saveEvent, uploadCloud, uploadVideo bool, recvTime time.Time, videoDuration int64) error {
	var personEvent, vehicleEvent, motorCycleEvent string

	switch notification.SrcName {
	case EventEnterArea:
		personEvent = cloud.FaceTracking
		vehicleEvent = cloud.LicensePlate
		motorCycleEvent = cloud.MotorCycleEnter
	case EventIntrusion:
		personEvent = cloud.Intrude
		vehicleEvent = cloud.Car
		motorCycleEvent = cloud.MotorCycleIntrude
	case EventCrossLine:
		personEvent = cloud.Intrude
		vehicleEvent = cloud.Car
		motorCycleEvent = cloud.MotorCycleIntrude
	default:
		return fmt.Errorf("unsupported event type")
	}

	if notification.StructureInfo.ObjInfo.PersonNum > 0 {
		u.assembleEvent(univCam, notification.SrcName, personEvent, notification.Timestamp,
			saveEvent, uploadCloud, uploadVideo,
			notification.StructureInfo.ImageInfoList, notification.StructureInfo.ObjInfo.GetPersonListObject(), recvTime, videoDuration)
	}
	if notification.StructureInfo.ObjInfo.VehicleNum > 0 {
		u.assembleEvent(univCam, notification.SrcName, vehicleEvent, notification.Timestamp,
			saveEvent, uploadCloud, uploadVideo,
			notification.StructureInfo.ImageInfoList, notification.StructureInfo.ObjInfo.GetVehicleListObject(), recvTime, videoDuration)
	}
	if notification.StructureInfo.ObjInfo.NonMotorVehicleNum > 0 {
		u.assembleEvent(univCam, notification.SrcName, motorCycleEvent, notification.Timestamp,
			saveEvent, uploadCloud, uploadVideo,
			notification.StructureInfo.ImageInfoList, notification.StructureInfo.ObjInfo.GetMotorCycleListObject(), recvTime, videoDuration)
	}
	return nil
}

func (u *UniviewAPI) handlerCloudDetectEvent(univCam *uniview.BaseUniviewCamera, eventTime int64, imageInfoList []ImageInfo,
	saveEvent, uploadCloud, uploadVideo bool, recvTime time.Time, videoDuration int64) (string, error) {
	if len(imageInfoList) == 0 {
		u.Logger.Error().Msg("image info list is null")
		return "", fmt.Errorf("onvif event no image")
	}
	img := imageInfoList[0]
	remoteID := u.processEvent(univCam, cloud.MotionStart, eventTime, img.Data, &structs.MetaScanData{}, saveEvent, uploadCloud, uploadVideo, recvTime, videoDuration)
	return remoteID, nil
}

func (u UniviewAPI) isUploadVideoBaseStream(univCam *uniview.BaseUniviewCamera) (bool, error) {
	localStreamSettings := univCam.GetLocalStreamSettings()
	if localStreamSettings == nil {
		u.Logger.Error().Msg("not found stream setting,upload video is false")
		return false, nil
	}
	for _, vsi := range localStreamSettings.VideoStreamInfos {
		// main stream less config max resolution and is H264
		resolution := vsi.VideoEncodeInfo.Resolution.Height * vsi.VideoEncodeInfo.Resolution.Width
		if int(vsi.ID) == 0 && vsi.MainStreamType == 0 &&
			resolution < u.Box.GetConfig().ThirdCameraMaxResolution() &&
			vsi.VideoEncodeInfo.EncodeFormat == 1 {
			return true, nil
		}
	}
	u.Logger.Error().Msg("upload video is false")
	return false, nil
}

func (u *UniviewAPI) handlerOtherEvent(univCam *uniview.BaseUniviewCamera, eventTime int64, imageInfoList []ImageInfo,
	saveEvent, uploadCloud, uploadVideo bool, recvTime time.Time, videoDuration int64) (remoteID string, err error) {
	detectParams := univCam.GetDetectParams()

	if uploadVideo {
		isUploadVideo, err := u.isUploadVideoBaseStream(univCam)
		if err == nil {
			uploadVideo = isUploadVideo
		}
	}

	switch detectParams.DetectAt {
	case model.DetectAtBox:
		err = u.handlerLocalDetectEvent(univCam, eventTime, imageInfoList, saveEvent, uploadCloud, uploadVideo, recvTime, videoDuration)
	case model.DetectAtCloud:
		remoteID, err = u.handlerCloudDetectEvent(univCam, eventTime, imageInfoList, saveEvent, uploadCloud, uploadVideo, recvTime, videoDuration)
	}
	return
}

func (u *UniviewAPI) HandleEventNotification(notification EventNotification, recvTime time.Time) error {
	univCam, err := u.getCamera(parseNVRSN(notification.Reference), notification.SrcID)
	if err != nil {
		return err
	}

	cfg := u.Box.GetConfig()
	if (notification.Timestamp - univCam.GetLastEventDetectTime()) < cfg.GetEventIntervalSecs() {
		u.Logger.Warn().Msgf("Filter this event,interval secs is %d(s), this trigger time is %d, previous event "+
			"trigger time is %d", cfg.GetEventIntervalSecs(), notification.Timestamp, univCam.GetLastEventDetectTime())
		return nil
	}
	univCam.SetLastEventDetectTime(notification.Timestamp)
	saveEvent := cfg.GetEventSavedHours() > 0
	uploadCloud := !cfg.GetDisableCloud()
	uploadVideo := univCam.GetUploadVideoEnabled()
	videoDuration := cfg.GetVideoClipDuration()
	if univCam.GetManufacturer() == utils.TuringUniview {
		return u.handleStandardEvent(univCam, notification, saveEvent, uploadCloud, uploadVideo, recvTime, videoDuration)
	} else {
		_, err = u.handlerOtherEvent(univCam, notification.Timestamp, notification.StructureInfo.ImageInfoList, saveEvent, uploadCloud, uploadVideo, recvTime, videoDuration)
		return err
	}
}

func (u UniviewAPI) saveEventToDB(camSN string, cameraID int, eventType string, imgBase64 string,
	meta *structs.MetaScanData, recvTime int64, now time.Time) (uint, error) {

	fs := db.FaceScan{
		Rect: db.FaceRect{
			Height: 0,
			Width:  0,
		},
		ImgBase64: imgBase64,
		Format:    "JPEG",
	}
	u.Logger.Info().Msg("saving picture to db")
	picInfo, err := u.DB.SavePictureToDB(&fs)
	if err != nil {
		u.Logger.Error().Err(err).Msg("failed to save pic to db")
		return 0, err
	}
	dataJSON, _ := json.Marshal(meta)

	event := &db.Event{
		CameraSN:    camSN,
		CameraID:    int64(cameraID),
		StartedAt:   now,
		EndedAt:     now,
		IPCTime:     time.Unix(recvTime, 0),
		CreatedAt:   now,
		UpdatedAt:   now,
		Type:        eventType,
		PictureID:   picInfo.ID,
		PictureHash: fmt.Sprintf("%x", md5.Sum([]byte(imgBase64))),
		Data:        string(dataJSON),
	}

	err = u.DB.CreateEvent(event)
	if err != nil {
		u.Logger.Error().Err(err).Msg("failed to save event to db")
		return 0, err
	}

	return event.ID, nil
}

func (u UniviewAPI) uploadEventToCloud(imgBase64 string, cameraID int, eventType string,
	now time.Time, eventTime time.Time, meta *structs.MetaScanData) (string, error) {
	faceScan := &db.FaceScan{
		ImgBase64: imgBase64,
		Format:    "JPEG",
		Rect:      db.FaceRect{},
	}

	f, err := u.uploadFileToS3(cameraID, faceScan)
	if err != nil {
		return "", err
	}
	return u.Box.UploadAICameraEvent(cameraID, f, now, now.Add(time.Second*1), eventTime, eventType, meta)
}

// Uniview IPCs send us coordinates in the range of [0,10000]. We need to convert this to pixel values.
func (u *UniviewAPI) convertCoordinates(w, h int, meta *structs.MetaScanData) {
	picHeightRatio, picWidthRatio := float64(h)/10000.0, float64(w)/10000.0
	for i, _ := range meta.Objects {
		bbox := meta.Objects[i].BBox
		bbox.X = bbox.X * picWidthRatio
		bbox.Y = bbox.Y * picHeightRatio
		bbox.Width = bbox.Width * picWidthRatio
		bbox.Height = bbox.Height * picHeightRatio
		meta.Objects[i].BBox = bbox
	}
	for i, polygonInfo := range meta.PolygonInfos {
		for j, point := range polygonInfo.Points {
			point.X = point.X * picWidthRatio
			point.Y = point.Y * picHeightRatio
			polygonInfo.Points[j] = point
		}
		meta.PolygonInfos[i] = polygonInfo
	}
}

func (u UniviewAPI) uploadFileToS3(cameraID int, face *db.FaceScan) (*utils.S3File, error) {
	filename, img, err := utils.SaveImage(u.Box.GetConfig().GetDataStoreDir(), face.ImgBase64)
	if err != nil {
		return nil, err
	}
	face.Rect = db.FaceRect{
		Height: img.Bounds().Dy(),
		Width:  img.Bounds().Dx(),
	}

	u.Logger.Debug().Str("filename", filename).Msg("Saved temp image file")
	defer func() {
		if err := os.Remove(filename); err != nil {
			u.Logger.Warn().Err(err).Str("filename", filename).Msg("unable to delete temp image file")
		}
	}()
	return u.Box.UploadS3ByTokenName(cameraID, filename, face.Rect.Height, face.Rect.Width, "jpg", box.TokenNameCameraEvent)
}

// TODO refactor upload media file
func (u UniviewAPI) handleEventVideo(remoteID string, camID int, startTime, endTime int64) (string, *utils.S3File, error) {
	// record video first
	baseCam, err := u.Box.GetCamera(camID)
	if err != nil {
		u.Logger.Error().Err(err).Msgf("camera %d not found, aborting", camID)
		return "", nil, err
	}
	cam, ok := baseCam.(base.AICamera)
	if !ok {
		u.Logger.Error().Msgf("camera %d not found, aborting", camID)
		return "", nil, err
	}
	videoName := fmt.Sprintf("%s.mp4", remoteID)
	resolution, streamId := utils.Normal, 2
	if cam.GetManufacturer() != utils.TuringUniview {
		resolution = utils.HD
		streamId = 1
	}

	cfg := u.Box.GetConfig().GetCameraConfig()
	delay, maxRetry := cfg.VideoUploadDelaySecs, cfg.VideoUploadMaxRetry
	for i := 0; i < maxRetry; i++ {
		err = cam.NvrWriteCacheToDisk(cam.GetChannel(), streamId, startTime, endTime)
		if err == nil {
			break
		}
		u.Logger.Error().Str("remoteID", remoteID).Msgf("failed to write nvr cache to disk: %s, try times: %d", err, i+1)
		time.Sleep(time.Duration(delay) * time.Second)
	}

	videoPath, width, height, err := cam.RecordVideo(string(resolution), startTime, endTime, videoName, false, configs.NormalDownloadSpeed)
	if err != nil {
		u.Logger.Error().Msgf("camera %d record video error %v, aborting", camID, err)
		return "", nil, err
	}

	// get resolution and codec_type by camID
	localSettings := cloud.GetCameraSettingsByID(cam.GetID())
	codecType := utils.CodecTypeH264
	resolutionId := utils.GetResolutionIdByString(string(resolution))
	if localSettings != nil {
		for _, vsi := range localSettings.StreamSettings {
			if resolutionId == int(vsi.ID) {
				width = int(vsi.VideoEncodeInfo.Resolution.Width)
				height = int(vsi.VideoEncodeInfo.Resolution.Height)
				codecType = utils.GetCodecTypeIdByID(vsi.VideoEncodeInfo.EncodeFormat)
				break
			}
		}
	}

	var s3File *utils.S3File
	var finalErr error
	for i := 0; i < 3; i++ {
		// upload video to s3
		finalErr = nil
		u.Logger.Debug().Msgf("uniview try upload video to s3, camera_id: %d, videopath: %s, time: %d", baseCam.GetID(), videoPath, i+1)
		s3File, err = u.Box.UploadS3ByTokenName(baseCam.GetID(), videoPath, height, width, "mp4", box.TokenNameCameraEvent)
		if err != nil {
			finalErr = fmt.Errorf("video file upload s3 error: %v", err)
			time.Sleep(time.Second * time.Duration(i+1))
			u.Logger.Error().Err(finalErr).Msgf("upload media file to s3 error %d", baseCam.GetID())
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
					CodecType:   codecType,
				},
				Key:    s3File.Key,
				Bucket: s3File.Bucket,
			},
			StartedAt: time.Unix(startTime, 0).Format(utils.CloudTimeLayout),
			EndedAt:   time.Unix(endTime, 0).Format(utils.CloudTimeLayout),
		})
		u.Logger.Debug().Msgf("uniview try upload media: %s to event: %s, camera_id: %d, time: %d", s3File.Key, remoteID, baseCam.GetID(), i+1)
		err = u.Box.UploadEventMedia(remoteID, &cloud.Media{
			Videos: &videos,
		})

		if err != nil {
			finalErr = fmt.Errorf("upload event media failed: %v", err)
			time.Sleep(time.Second * time.Duration(i+1))
			u.Logger.Error().Err(finalErr).Msgf("upload event media to cloud error %d", baseCam.GetID())
			continue
		}
		break
	}

	// do clean when success
	if len(videoPath) > 0 && finalErr == nil {
		if err := os.Remove(videoPath); err != nil {
			u.Logger.Warn().Err(err).Str("filename", videoPath).Msg("unable to delete temp image file")
		}
	}
	return videoPath, s3File, finalErr
}

func (u *UniviewAPI) getPolygonInfos(univCam *uniview.BaseUniviewCamera, srcName, eventType string) []structs.PolygonInfo {
	polygonInfos := make([]structs.PolygonInfo, 0)
	oriPolygonInfos := make([]structs.PolygonInfo, 0)
	client := u.DB.GetDBInstance()
	rule, err := db.GetRule(client, univCam.GetID())
	if err != nil {
		return polygonInfos
	}
	switch srcName {
	case EventEnterArea:
		oriPolygonInfos = getPolygonFromRule(rule.EnterArea)
	case EventIntrusion:
		oriPolygonInfos = getPolygonFromRule(rule.Intrusion)
	}
	for _, v := range oriPolygonInfos {
		switch eventType {
		case cloud.Intrude, cloud.FaceTracking:
			if lo.Contains(v.EnabledTypes, uniview.Pedestrian) {
				polygonInfos = append(polygonInfos, v)
			}
		case cloud.Car, cloud.LicensePlate:
			if lo.Contains(v.EnabledTypes, uniview.MotorVehicle) {
				polygonInfos = append(polygonInfos, v)
			}
		case cloud.MotorCycleEnter, cloud.MotorCycleIntrude:
			if lo.Contains(v.EnabledTypes, uniview.NonMotorVehicle) {
				polygonInfos = append(polygonInfos, v)
			}
		}
	}
	return polygonInfos
}

func getPolygonFromRule(data string) []structs.PolygonInfo {
	polygonInfos := make([]structs.PolygonInfo, 0)
	if len(data) == 0 {
		return polygonInfos
	}
	area := goshawk.Area{}
	if err := json.Unmarshal([]byte(data), &area); err != nil {
		return polygonInfos
	}
	for _, polygonInfo := range area.PolygonInfoList {
		pointList := make([]structs.Point, 0)
		for _, point := range polygonInfo.PointList {
			pointList = append(pointList, structs.Point{
				X: float64(point.X),
				Y: float64(point.Y),
			})
		}
		enableTypes := make([]int, 0)
		for _, detect := range polygonInfo.DetectTargetList {
			if detect.Enabled == uniview.DetectEnable {
				enableTypes = append(enableTypes, detect.Type)
			}
		}
		polygonInfos = append(polygonInfos, structs.PolygonInfo{Points: pointList, EnabledTypes: enableTypes})
	}
	return polygonInfos
}

func convertUniviewOSDTextArea(areas []uniview.OSDTextArea) []structs.OSDTextArea {
	apiOSDTextAreas := make([]structs.OSDTextArea, 0)
	for _, area := range areas {
		apiOSDTextArea := structs.OSDTextArea{Points: make([]structs.Point, 0)}
		for _, point := range area.Points {
			apiOSDTextArea.Points = append(apiOSDTextArea.Points, structs.Point{X: point.X, Y: point.Y})
		}
		apiOSDTextAreas = append(apiOSDTextAreas, apiOSDTextArea)
	}
	return apiOSDTextAreas
}

func (u *UniviewAPI) getSnapshotOSDTextAreas(cam *uniview.BaseUniviewCamera) []structs.OSDTextArea {
	return convertUniviewOSDTextArea(cam.GetSnapshotOSDTextAreas())
}

func (u *UniviewAPI) getVideoOSDTextAreas(cam *uniview.BaseUniviewCamera) []structs.OSDTextArea {
	return convertUniviewOSDTextArea(cam.GetVideoOSDTextAreas())
}
