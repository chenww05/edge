package sunell

import (
	"net/http"
	"strings"

	"github.com/codegangsta/inject"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/turingvideo/minibox/box"
	"github.com/turingvideo/minibox/db"
	"github.com/turingvideo/minibox/utils"
	http2 "github.com/turingvideo/turing-common/http"
	"github.com/turingvideo/turing-common/log"
)

// for camera
type SunellAPI struct {
	Box    box.Box   `inject:"box"`
	DB     db.Client `inject:"db"`
	Logger zerolog.Logger
}

func Register(injector inject.Injector, router *gin.Engine) {
	logger := log.Logger("sunell_api")
	api := &SunellAPI{Logger: logger}
	if err := injector.Apply(api); err != nil {
		logger.Fatal().Err(err).Msg("Failed to init sunell api.")
	}
	http2.RegisterGinGroupHandler(&router.RouterGroup, api)
}

func (c *SunellAPI) BaseURL() string {
	return "api/sunell"
}

func (c *SunellAPI) Middlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{}
}

func (c *SunellAPI) Register(group *gin.RouterGroup) {
	group.POST("/upload_event/:extend", c.UploadEvent)
	group.POST("/heartbeat", c.Heartbeat)
}

func (c *SunellAPI) UploadEvent(ctx *gin.Context) {
	data, heartbeat, err := c.parseBody(ctx)
	if err != nil {
		c.Logger.Error().Err(err).Msg("[Error Request] cannot parse request body")
		ctx.JSON(http.StatusOK, nil)
		return
	}
	if heartbeat {
		c.Logger.Info().Msg("Sunell heartbeat")
		ctx.JSON(http.StatusOK, nil)
		return
	}

	picType := data[AIPictureType]
	if picType == PictureTypePart {
		ctx.JSON(http.StatusOK, nil)
		return
	}

	if err := c.HandleUploadEvent(data); err != nil {
		c.Logger.Error().Err(err).Msg("upload event handler error")
	}
	ctx.JSON(http.StatusOK, nil)
}

func (c *SunellAPI) Heartbeat(ctx *gin.Context) {
	var heartbeat HeartbeatInfo
	err := ctx.ShouldBindJSON(&heartbeat)
	if err != nil {
		c.Logger.Error().Msgf("Heartbeat request error: %s", err)
		ctx.JSON(http.StatusBadRequest, utils.ErrParameters)
		return
	}

	if heartbeat.MAC == "" {
		c.Logger.Error().Str("error", "heartbeat.MAC is empty").Send()
		ctx.JSON(http.StatusBadRequest, utils.ErrParameters)
		return
	}

	if heartbeat.IPV4 == "" {
		c.Logger.Error().Str("error", "heartbeat.IPV4 is empty").Send()
		ctx.JSON(http.StatusBadRequest, utils.ErrParameters)
		return
	}

	mac := strings.ReplaceAll(heartbeat.MAC, ":", "")
	sn := mac[len(mac)-6:]
	camera, err := c.getCameraBySN(sn)

	if err != nil {
		if err.Error() == CameraBrandMissMatchErr {
			c.Logger.Error().Err(err).Send()
			ctx.JSON(http.StatusInternalServerError, utils.ErrUnknown(err))
			return
		}

		c.syncTimeZone(camera)
		c.Box.AddCamera(camera)
	} else {
		camera.Heartbeat(heartbeat.IPV4, DefaultCameraVerson)
		c.syncTimeZone(camera)
	}

	ctx.JSON(http.StatusOK, utils.ErrOK(nil, 0, 0, 0))
}
