package guardian

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/codegangsta/inject"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/example/minibox/box"
	"github.com/example/minibox/db"
	"github.com/example/minibox/utils"
	http2 "github.com/example/turing-common/http"
	"github.com/example/turing-common/log"
)

type GuardianAPI struct {
	Box    box.Box   `inject:"box"`
	DB     db.Client `inject:"db"`
	logger zerolog.Logger
}

func Register(injector inject.Injector, router *gin.Engine) {
	logger := log.Logger("guardian_api")
	api := &GuardianAPI{logger: logger}
	if err := injector.Apply(api); err != nil {
		logger.Fatal().Err(err).Msg("Failed to init guardian api.")
	}
	http2.RegisterGinGroupHandler(&router.RouterGroup, api)
}

func (u *GuardianAPI) BaseURL() string {
	return "api/v1"
}

func (u *GuardianAPI) Middlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		// u.ShowContent, // for debug
	}
}

func (u *GuardianAPI) Register(group *gin.RouterGroup) {
	group.PATCH("/camera/cameras/:id", u.UpdateCamera)
	group.POST("/event/events", u.UploadEvent)
	group.POST("/medium/mediums", u.UploadMedium)
}

func (u *GuardianAPI) UpdateCamera(ctx *gin.Context) {
	id := ctx.Param("id")
	u.logger.Info().Msgf("camera update: %s", id)
	ctx.JSON(http.StatusOK, utils.TVResponseOK(map[string]interface{}{"id": id}))
}

func (u *GuardianAPI) UploadEvent(ctx *gin.Context) {
	var req Event
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		u.logger.Error().Msgf("UploadEvent request error: %s", err)
		ctx.JSON(http.StatusBadRequest, utils.ErrParameters)
		return
	}
	req.Check()
	// handle upload event
	if err := u.handleUploadEvent(req); err != nil {
		u.logger.Error().Msgf("UploadEvent error: %s", err)
		ctx.JSON(http.StatusBadRequest, utils.ErrParameters)
		return
	}
	ctx.JSON(http.StatusCreated, utils.TVResponseOK(map[string]interface{}{}))
}

func (u *GuardianAPI) UploadMedium(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.TVResponseErr(2001, "", map[string]interface{}{}))
		return
	}
	tmp := strings.Split(file.Filename, "/")
	if len(tmp) == 0 {
		ctx.JSON(http.StatusBadRequest, utils.TVResponseErr(2001, "", map[string]interface{}{}))
		return
	}
	fileName := tmp[len(tmp)-1]
	filePath := fmt.Sprintf("%s/%s", u.Box.GetConfig().GetDataStoreDir(), fileName)
	if err := ctx.SaveUploadedFile(file, filePath); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.TVResponseErr(2001, "", map[string]interface{}{}))
		return
	}
	name, ok := ctx.GetPostForm("name")
	if !ok {
		ctx.JSON(http.StatusBadRequest, utils.TVResponseErr(2001, "", map[string]interface{}{}))
		return
	}

	medium := &db.Medium{
		Name:     name,
		FilePath: filePath,
	}
	if err = u.DB.CreateMedium(medium); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.TVResponseErr(2001, "", map[string]interface{}{}))
		return
	}
	ctx.JSON(http.StatusCreated, utils.TVResponseOK(map[string]interface{}{"id": medium.ID}))
}
