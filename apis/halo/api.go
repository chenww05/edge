package halo

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/codegangsta/inject"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/turingvideo/minibox/box"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/db"
	"github.com/turingvideo/minibox/utils"
	http2 "github.com/turingvideo/turing-common/http"
	"github.com/turingvideo/turing-common/log"
)

type HaloAPI struct {
	Box    box.Box   `inject:"box"`
	DB     db.Client `inject:"db"`
	Logger zerolog.Logger
}

func Register(injector inject.Injector, router *gin.Engine) {
	logger := log.Logger("halo_api")
	api := &HaloAPI{Logger: logger}
	if err := injector.Apply(api); err != nil {
		logger.Fatal().Err(err).Msg("Failed to init halo api.")
	}
	http2.RegisterGinGroupHandler(&router.RouterGroup, api)
}

func (h *HaloAPI) BaseURL() string {
	return "halo"
}

// Middlewares TODO: Add authorization for halo events
func (h *HaloAPI) Middlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{}
}

func (h *HaloAPI) Register(group *gin.RouterGroup) {
	group.POST("/event", h.UploadEvent)
	group.GET("/heartbeat", h.UploadStatus)
}

func (h *HaloAPI) UploadEvent(ctx *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			h.Logger.Error().Msgf("panic from upload halo event %v", err)
		}
	}()
	defer func() {
		ctx.JSON(http.StatusOK, nil)
	}()

	rawData, err := ctx.GetRawData()
	if err != nil {
		h.Logger.Error().Msgf("ctx.GetRawData error: %s", err)
		return
	}
	h.Logger.Info().RawJSON("data", rawData).Msg("UploadHaloEvent")
	event := HaloEventNotification{}
	if err = json.Unmarshal(rawData, &event); err != nil {
		h.Logger.Error().Msgf("Unmarshal HaloEventNotification error: %s", err)
		return
	}

	eventType := getHaloEventType(event.EventType)
	if eventType == cloud.HaloEventUnknown {
		h.Logger.Error().Msg("unknown halo event type error")
		return
	}
	startTime := time.Now().Format(utils.CloudTimeLayout)
	var eventInfo = cloud.HaloEventInfo{
		Source:       cloud.EventSourceHalo,
		BoxId:        h.Box.GetBoxId(),
		IotDeviceMAC: event.MAC,
		StartedAt:    startTime,
		Metadata: cloud.HaloMetaData{
			EventType:   eventType,
			Threshold:   event.Threshold,
			SensorValue: event.SensorValue,
			DataSource:  event.DataSource,
		},
	}
	if err = h.Box.CloudClient().UploadHaloEvent(&eventInfo); err != nil {
		h.Logger.Err(err).Msgf("failed to upload halo event info to broadway")
	}
}

func (h *HaloAPI) UploadStatus(ctx *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			h.Logger.Error().Msgf("panic from upload halo status %v", err)
		}
	}()
	defer func() {
		ctx.JSON(http.StatusOK, nil)
	}()

	rawData, err := ctx.GetRawData()
	if err != nil {
		h.Logger.Error().Msgf("ctx.GetRawData error: %s", err)
		return
	}
	h.Logger.Info().RawJSON("data", rawData).Msg("UploadHaloHeartbeat")
	hb := HaloHeartbeat{}
	if err = json.Unmarshal(rawData, &hb); err != nil {
		h.Logger.Error().Msgf("Unmarshal HaloHeartbeat error: %s", err)
		return
	}
	activeList := strings.Split(hb.Active, ",")
	timestamp := time.Now().Format(utils.CloudTimeLayout)
	var heartbeat = cloud.HaloHeartbeat{
		Source:       cloud.EventSourceHalo,
		BoxId:        h.Box.GetBoxId(),
		IotDeviceMAC: hb.MAC,
		Timestamp:    timestamp,
		Sensors:      createSensorList(hb.Sensors, activeList),
	}

	if err = h.Box.CloudClient().UploadHaloHeartbeat(&heartbeat); err != nil {
		h.Logger.Err(err).Msgf("failed to upload halo heartbeat info to broadway")
	}

}

