package uniview

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/codegangsta/inject"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/example/minibox/box"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/db"
	"github.com/example/minibox/utils"
	http2 "github.com/example/turing-common/http"
	"github.com/example/turing-common/log"
)

type UniviewAPI struct {
	Box           box.Box   `inject:"box"`
	DB            db.Client `inject:"db"`
	motionProcess MotionProcess
	Logger        zerolog.Logger
}

func Register(injector inject.Injector, router *gin.Engine) {
	logger := log.Logger("uniview_api")
	api := &UniviewAPI{
		motionProcess: newMotionProcess(),
		Logger:        logger,
	}
	if err := injector.Apply(api); err != nil {
		logger.Fatal().Err(err).Msg("Failed to init uniview api.")
	}
	http2.RegisterGinGroupHandler(&router.RouterGroup, api)
}

func (u UniviewAPI) BaseURL() string {
	return "LAPI/V1.0"
}

func (u UniviewAPI) Middlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		// u.ShowContent, // for debug
	}
}

func (u UniviewAPI) Register(group *gin.RouterGroup) {
	group.POST("/System/Event/Notification/Structure", u.UploadEvent)
	group.POST("/System/Event/Notification/Alarm", u.UploadAlarm)
	group.POST("/System/Event/Notification/MotionDetection", u.UploadMotionDetection)
}

func (u UniviewAPI) ShowContent(ctx *gin.Context) {
	raw, err := ctx.GetRawData()
	if err != nil {
		u.Logger.Error().Err(err)
		return
	}
	u.Logger.Debug().Msgf("%s", string(raw))

	ctx.Request.Body = ioutil.NopCloser(bytes.NewBuffer(raw))
	ctx.Next()
}

func (u *UniviewAPI) LogObject(notification EventNotification) {
	jsonBytes, _ := json.Marshal(notification.StructureInfo.ObjInfo)
	u.Logger.Debug().RawJSON("objects", jsonBytes).Msgf("[%d][%s][%d][%s]", notification.Timestamp, parseNVRSN(notification.Reference), notification.SrcID, notification.SrcName)
}

func (u *UniviewAPI) UploadEvent(ctx *gin.Context) {
	eventRecvTime := time.Now().UTC()
	u.Logger.Debug().Str("url", ctx.Request.RequestURI).Msg("UploadEvent")
	notification := EventNotification{}
	rawData, err := ctx.GetRawData()
	if err != nil {
		u.Logger.Error().Msgf("ctx.GetRawData error: %s", err)
		ctx.JSON(http.StatusOK, nil)
		return
	}
	if err := json.Unmarshal(rawData, &notification); err != nil {
		u.Logger.Error().Msgf("Unmarshal EventNotification error: %s", err)
		ctx.JSON(http.StatusOK, nil)
		return
	}
	u.LogObject(notification)

	err = u.HandleEventNotification(notification, eventRecvTime)

	if err != nil {
		u.Logger.Error().Str("nvrSn", parseNVRSN(notification.Reference)).Uint32("channel", notification.SrcID).Msgf("Handle Uniview Event error: %s", err)
		ctx.JSON(http.StatusOK, nil)
		return
	}
	ctx.JSON(http.StatusOK, nil)
}

// UploadAlarm api
func (u *UniviewAPI) UploadAlarm(ctx *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			u.Logger.Error().Msgf("panic from upload alarm %v", err)
		}
	}()
	defer func() {
		ctx.JSON(http.StatusOK, nil)
	}()
	rawData, err := ctx.GetRawData()
	if err != nil {
		u.Logger.Error().Msgf("ctx.GetRawData error: %s", err)
		return
	}
	u.Logger.Info().RawJSON("data", rawData).Msgf("UploadAlarm")
	alarm := AlarmNotification{}
	if err = json.Unmarshal(rawData, &alarm); err != nil {
		u.Logger.Error().Msgf("Unmarshal AlarmNotification error: %s", err)
		return
	}

	nvrSN := parseNVRSN(alarm.Reference)
	if len(nvrSN) == 0 {
		u.Logger.Error().Msgf("parse nvr sn null, reference is %s", alarm.Reference)
		return
	}
	univCam, err := u.getCamera(nvrSN, uint32(alarm.AlarmInfo.AlarmSrcID))
	if err != nil {
		u.Logger.Err(err).Msgf("nvr sn is %s, source id is %d", nvrSN, alarm.AlarmInfo.AlarmSrcID)
		return
	}

	if alarm.AlarmInfo.AlarmType == MotionAlarmOff {
		events := u.motionProcess.popAllMotions(univCam.GetID())
		u.Logger.Debug().Msgf("pop all motions length %d", len(events))
		if len(events) == 0 {
			return
		}
		// TODO may delete this logic
		if u.Box.GetConfig().GetEnableMotionVideo() {
			if err = u.motionProcess.uploadMotionEventsVideo(events, u, univCam); err != nil {
				u.Logger.Err(err).Msgf("upload motion events video error %v", univCam.ID)
			}
		}
		return
	}

	atime := time.Now().Format(utils.CloudTimeLayout)
	ctime := time.Unix(alarm.AlarmInfo.Timestamp, 0).Format(utils.CloudTimeLayout)
	var alarmInfo = cloud.AlarmInfo{
		Source:    cloud.AlarmSourceBridge,
		BoxId:     u.Box.GetBoxId(),
		CameraId:  univCam.ID,
		StartedAt: atime,
		EndedAt:   atime,
		IPCTime:   ctime,
	}
	switch alarm.AlarmInfo.AlarmType {
	case VideoLossAlarmOn:
		alarmInfo.Detection = cloud.Detection{
			Algos: cloud.AlarmTypeVideoLossStarted,
		}
	case VideoTamperingOn:
		alarmInfo.Detection = cloud.Detection{
			Algos: cloud.AlarmTypeVideoTamperStarted,
		}
	case IPCOffline:
		alarmInfo.Detection = cloud.Detection{
			Algos: cloud.AlarmTypeIPCOffline,
		}
	default:
		return
	}
	if err = u.Box.CloudClient().UploadAlarmInfo(&alarmInfo); err != nil {
		u.Logger.Err(err).Msgf("failed to upload alarm info to broadway")
	}
}

func (u *UniviewAPI) UploadMotionDetection(ctx *gin.Context) {
	defer func() {
		ctx.JSON(http.StatusOK, nil)
	}()
	if u.Box.GetConfig().GetDisableMotionEvent() {
		u.Logger.Info().Msgf("disable motion event")
		return
	}
	rawData, err := ctx.GetRawData()
	if err != nil {
		u.Logger.Error().Msgf("ctx.GetRawData error: %s", err)
		ctx.JSON(http.StatusOK, nil)
		return
	}

	mda := MotionDetectionAlarm{}
	if err = json.Unmarshal(rawData, &mda); err != nil {
		u.Logger.Error().Msgf("Unmarshal MotionDetection error: %s,data %s", err, rawData)
		ctx.JSON(http.StatusOK, nil)
		return
	}
	if len(mda.AlarmPicture.ImageList) == 0 {
		u.Logger.Error().Msgf("this event no images %v", mda)
		return
	}
	nvrSN := parseNVRSN(mda.Reference)
	if len(nvrSN) == 0 {
		u.Logger.Error().Msgf("parse nvr sn null, reference is %s", mda.Reference)
		return
	}
	univCam, err := u.getCamera(nvrSN, mda.SourceID)
	if err != nil {
		u.Logger.Err(err).Msgf("nvr sn is %s, source id is %d", nvrSN, mda.SourceID)
		return
	}
	if univCam.GetManufacturer() == utils.TuringUniview {
		return
	}
	event := &motionEvent{
		cameraID:    univCam.GetID(),
		timestamp:   mda.Timestamp,
		imgBase64:   mda.AlarmPicture.ImageList[0].Data,
		receiveTime: time.Now().UTC(),
	}
	//if ok := u.motionProcess.isMotionStart(univCam.GetID()); !ok {
	//	u.Logger.Info().Msgf("this event not motion_start, is motion_in,sourceID:%d", mda.SourceID)
	//	u.motionProcess.pushMotion(event)
	//	return
	//}
	//u.motionProcess.pushMotion(event)
	go func() {
		if err = u.motionProcess.uploadMotionStartEvent(event, u, univCam, mda.AlarmPicture.ImageList); err != nil {
			u.Logger.Err(err).Msgf("uploadMotionEvent error, nvr sn is %s, source id is %d", nvrSN, mda.SourceID)
			return
		}
		u.Logger.Info().Msgf("processMotionEvent success, nvr sn is %s, source id is %d", nvrSN, mda.SourceID)
	}()
}
