package thermal_1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"time"

	"github.com/codegangsta/inject"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/example/minibox/apis/common"
	"github.com/example/minibox/apis/structs"
	"github.com/example/minibox/camera/thermal_1"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/db"
	"github.com/example/minibox/printer"
	"github.com/example/minibox/utils"
	http2 "github.com/example/turing-common/http"
	"github.com/example/turing-common/log"
)

type TemperatureQuery interface {
	GetTemperatureConfig() (*thermal_1.TemperatureConfig, error)
}

type Thermal1API struct {
	cmds   map[string]func(*gin.Context, []byte)
	logger zerolog.Logger
	cache  map[string]*faceCache
	Base   common.ThermalBaseAPI `inject:"base"`
}

func Register(injector inject.Injector, router *gin.Engine) {
	logger := log.Logger("thermal_1")
	t := &Thermal1API{
		cmds:   map[string]func(*gin.Context, []byte){},
		logger: logger,
		cache:  NewFaceCacheSet(),
	}
	t.registerCmds()
	if err := injector.Apply(t); err != nil {
		logger.Fatal().Err(err).Msg("Failed to init thermal cmds.")
	}
	http2.RegisterGinGroupHandler(&router.RouterGroup, t)
}

func (t *Thermal1API) BaseURL() string {
	return "/"
}

func (t *Thermal1API) Middlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{}
}

func (t *Thermal1API) Register(group *gin.RouterGroup) {
	group.POST("/", t.HandleCmd)
}

func (t *Thermal1API) HandleCmd(ctx *gin.Context) {
	defer ctx.Request.Body.Close()

	payload, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		t.logger.Error().Err(err).Msg("could not read scanner request body")
		ctx.AbortWithError(http.StatusOK, err)
		return
	}

	if json.Valid(payload) {
		t.logger.Debug().RawJSON("payload", payload).Msg("scanner command received")
	} else {
		t.logger.Debug().Str("payload", string(payload)).Msg("scanner command received")
	}

	var req thermal_1.BaseReq
	err = json.Unmarshal(payload, &req)
	if err != nil {
		t.logger.Error().Err(err).RawJSON("payload", payload).Msg("could not unmarshal command payload")
		ctx.AbortWithError(http.StatusOK, err)
		return
	}

	handle, ok := t.cmds[req.Cmd]
	if !ok {
		t.logger.Error().Msgf("Unsupported cmd: %s", req.Cmd)
		ctx.AbortWithStatus(http.StatusOK)
		return
	}
	handle(ctx, payload)
}

func (t *Thermal1API) registerCmds() {
	t.cmds[CmdFace] = t.handleFace
	t.cmds[CmdHeartBeat] = t.handleHeartbeat
	t.cmds[CmdQuestionnaire] = t.handleQuestionnaire
	t.cmds[CmdCardReader] = t.handleRfId
	t.cmds[CmdTimeClock] = t.handleTimeClock
	// ask_upgrade has no IP in it.
	//t.cmds["ask_upgrade"] = t.handleHeartbeat
}

func (t *Thermal1API) handleFace(ctx *gin.Context, payload []byte) {
	logger := t.logger.With().Str("cmd", "face").Int("scanner_gen", 1).Logger()

	var req thermal_1.FaceInfo
	if err := json.Unmarshal(payload, &req); err != nil {
		logger.Error().Err(err).RawJSON("payload", payload).Msg("failed to unmarshal payload")
		return
	}

	// There is no need to add heartbeat here as it's handled by HandleUploadEvent
	//cameraInfo, err := t.Base.Box.GetCameraBySN(req.SN)
	//if err != nil {
	//	t.logger.Error().Msgf("cannot find camera %s: %s", req.SN, err)
	//	return
	//}
	//cameraInfo.Heartbeat(cameraInfo.GetIP(), "")

	faceCache, exist := t.cache[req.SN]
	if !exist {
		logger.Info().Str("camera_sn", req.SN).Msg("serial number not found in cache")
		// No need to pass cache map into NewFaceCache.
		faceCache = NewFaceCache(t.Base)
		t.cache[req.SN] = faceCache
	}
	faceCache.Add(&req)
}

func (t *Thermal1API) handleHeartbeat(ctx *gin.Context, payload []byte) {
	logger := t.logger.With().Str("cmd", "heartbeat").Logger()

	var req thermal_1.HeartbeatInfo
	if err := json.Unmarshal(payload, &req); err != nil {
		logger.Error().Err(err).RawJSON("payload", payload).Msg("failed to unmarshal payload")
		return
	}

	v, ok := req.Version.(string)
	if !ok {
		logger.Debug().Msg("missing version number")
		v = ""
	}

	cam, err := t.Base.Box.GetCameraBySN(req.SN)
	if err != nil {
		t.logger.Info().Str("camera_sn", req.SN).Msg("camera not found in memory")
		cam := thermal_1.NewLiveCamera(req.SN, req.IP, v)
		t.Base.Box.AddCamera(cam)
	} else {
		t.logger.Info().Str("ip", req.IP).Str("version", v).Str("camera_sn", cam.GetSN()).Msg("updating camera information")
		cam.Heartbeat(req.IP, v)
	}
}

func (t *Thermal1API) handleQuestionnaire(ctx *gin.Context, payload []byte) {
	logger := t.logger.With().Str("cmd", "questionnaire").Int("scanner_gen", 2).Logger()

	var req thermal_1.QuestionnaireInfo
	if err := json.Unmarshal(payload, &req); err != nil {
		logger.Error().Err(err).RawJSON("payload", payload).Msg("failed to unmarshal payload")
		return
	}

	baseCam, err := t.Base.Box.GetCameraBySN(req.SN)
	if err != nil {
		logger.Error().Err(err).Str("camera_sn", req.SN).Msg("camera not found, aborting")
		return
	}

	cam, ok := baseCam.(TemperatureQuery)
	if !ok {
		logger.Error().Str("brand", string(baseCam.GetBrand())).Msg("camera has wrong type, aborting. check cloud configuration")
		return
	}

	tc, err := cam.GetTemperatureConfig()
	if err != nil {
		logger.Error().Err(err).Msg("could not get temperature settings from scanner, aborting")
		return
	}

	var limit utils.Celsius
	if tc.FahrenheitUnit {
		limit = utils.Fahrenheit(tc.Limit).ToC()
		logger.Debug().Float64("tc_limit", tc.Limit).Float64("limit", float64(limit)).Msg("converting temperature config limit from farenheight to celsius")
	} else {
		limit = utils.Celsius(tc.Limit)
	}

	meta := &structs.MetaScanData{
		Abnormal:            float64(limit),
		QuestionnaireResult: req.Pass,
		// Only Gen 2 has questionnaire.
		HasQuestionnaire: true,
		RfId:             req.RfId,
		PhoneNumber:      req.PhoneNumber,
		VisitingWho:      req.VisitingWho,
		Email:            req.Email,
		Site:             req.Site,
	}

	if req.Person != nil {
		req.Person.Temperature = math.Ceil(req.Person.Temperature*100) / 100
		meta.Temperature = req.Person.Temperature
	}

	if req.Match != nil {
		meta.Company, meta.PersonName = req.Match.Company, req.Match.PersonName
		meta.PersonId, meta.PersonRole = req.Match.PersonId, req.Match.PersonRole
	}

	if req.QrCode != nil {
		meta.QrCode = &structs.QrCode{
			QrType: req.QrCode.QrType,
			QrData: req.QrCode.QrData,
		}
	}

	if !req.Pass {
		if len(req.Company) > 0 {
			meta.Company = req.Company
		} else {
			meta.Company = req.PersonCompany
		}

		meta.PersonRole, meta.PersonId, meta.PersonName = req.PersonRole, req.PersonId, req.PersonName
	}

	event := common.ScanEvent{
		SN:           req.SN,
		PrintData:    toPrintData2(*tc, req, meta),
		MetaScanData: meta,
		Limit:        limit,
		Temperature:  utils.Celsius(meta.Temperature),
	}

	if req.Picture != nil {
		event.FaceScan = &db.FaceScan{
			ImgBase64: req.Picture.Data,
			Format:    req.Picture.Format,
			Rect: db.FaceRect{
				Height: req.Picture.FaceHeight,
				Width:  req.Picture.FaceWidth,
				X:      req.Picture.FaceX,
				Y:      req.Picture.FaceY,
			},
		}
	}

	if err := t.Base.HandleUploadEvent(event); err != nil {
		t.logger.Error().Err(err).Msg("upload questionnaire handler error")
	}
}

func toPrintData2(tc thermal_1.TemperatureConfig, info thermal_1.QuestionnaireInfo, meta *structs.MetaScanData) *printer.PrintData {
	var (
		overTemp bool
		photo    *string
		temp     utils.Celsius
	)

	if info.Person != nil {
		temp = utils.Celsius(info.Person.Temperature)

		if tc.FahrenheitUnit {
			overTemp = temp > utils.Fahrenheit(tc.Limit).ToC()
		} else {
			overTemp = temp > utils.Celsius(tc.Limit)
		}
	}

	if info.Picture != nil {
		photo = &info.Picture.Data
	}

	return &printer.PrintData{
		Temperature: temp,
		OverTemp:    overTemp,
		Photo:       photo,
		Meta:        meta,
	}
}

func (t *Thermal1API) handleRfId(ctx *gin.Context, payload []byte) {
	timeKeepingEnable := t.Base.Box.GetConfig().GetTimeKeepingEnable()
	if timeKeepingEnable {
		t.handleRfIdForTimeKeeping(ctx, payload)
	} else {
		t.handleRfIdForNormal(ctx, payload)
	}
}

func (t *Thermal1API) handleRfIdForNormal(ctx *gin.Context, payload []byte) {
	logger := t.logger.With().Str("cmd", "ping card reader").Logger()

	cameras := t.Base.Box.GetCamGroup().AllCameras()
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	for _, camera := range cameras {
		if !camera.GetOnline() {
			logger.Info().Str("camera_sn", camera.GetSN()).Msg("skipping offline camera")
			continue
		}

		url := fmt.Sprintf("http://%s:8000", camera.GetIP())

		res, err := client.Post(url, "application/json", bytes.NewReader(payload))
		if err != nil {
			logger.Error().Err(err).Str("camera_ip", camera.GetIP()).Str("camera_sn", camera.GetSN()).Msg("failed to forward card reader")
			continue
		}

		defer res.Body.Close()
	}
}

func (t *Thermal1API) handleRfIdForTimeKeeping(ctx *gin.Context, payload []byte) {
	logger := t.logger.With().Str("cmd", "ping card reader timekeeping").Logger()

	var req cloud.RfIdInfo
	if err := json.Unmarshal(payload, &req); err != nil {
		logger.Error().Err(err).Msg("failed to unmarshal payload, aborting")
		return
	}

	cameras := t.Base.Box.GetCamGroup().AllCameras()
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	// get employee info clock status from cloud
	employeeInfo, err := t.Base.Box.CloudClient().RecognizeRfId(&req)
	if err != nil {
		logger.Error().Err(err).Msg("RecognizeRfId failed, aborting")
		return
	}

	var clockStatus int
	switch employeeInfo.ClockStatus {
	case string(ClockStatusNew):
		clockStatus = 1
	case string(ClockStatusInProgress):
		clockStatus = 2
	case string(ClockStatusInBreak):
		clockStatus = 3
	default:
		logger.Error().Str("clock_status", employeeInfo.ClockStatus).Msg("unknown clock status, aborting")
		return
	}
	marshalled, err := json.Marshal(map[string]interface{}{
		"version":          CmdVersion,
		"cmd":              CmdCardReader,
		"pass":             true,
		"name":             employeeInfo.FullName,
		"id":               employeeInfo.NO,
		"rfid":             req.RfId,
		"company":          "",
		"head_shot":        employeeInfo.ImageBase64,
		"clock_req_status": clockStatus,
	})
	if err != nil {
		logger.Error().Err(err).Msg("could not marshal timekeeping request to scanner, aborting")
		return
	}

	for _, camera := range cameras {
		if !camera.GetOnline() {
			logger.Info().Str("camera_sn", camera.GetSN()).Msg("skipping offline camera")
			continue
		}

		url := fmt.Sprintf("http://%s:8000", camera.GetIP())

		res, err := client.Post(url, "application/json", bytes.NewReader(marshalled))
		if err != nil {
			logger.Error().Err(err).Str("camera_ip", camera.GetIP()).Str("camera_sn", camera.GetSN()).Msg("failed to forward card reader")
			continue
		}

		defer res.Body.Close()
	}
}

func (t *Thermal1API) handleTimeClock(ctx *gin.Context, payload []byte) {
	logger := t.logger.With().Str("cmd", "time clock").Logger()

	var timeClock TimeClockInfo
	if err := json.Unmarshal(payload, &timeClock); err != nil {
		logger.Error().Err(err).RawJSON("payload", payload).Msg("failed to unmarshal payload, aborting")
		return
	}

	var action TimeClockAction
	switch timeClock.Action {
	case 1:
		action = ActionClockIn
	case 2:
		action = ActionClockOut
	case 3:
		action = ActionStartBreak
	case 4:
		action = ActionEndBreak
	default:
		t.logger.Error().Int("action", timeClock.Action).Msg("clock time action not recognized, aborting")
		return
	}

	// need to send response to device
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	camera, err := t.Base.Box.GetCameraBySN(timeClock.SN)
	if err != nil {
		logger.Error().Err(err).Str("camera_sn", timeClock.SN).Msg("cannot find camera in memory, aborting")
		return
	}
	cameraUrl := fmt.Sprintf("http://%s:8000", camera.GetIP())

	req := cloud.ClockRecord{
		TimeStamp:  timeClock.TimeStamp,
		EmployeeID: timeClock.EmployeeID,
		RfId:       timeClock.RfId,
		Action:     string(action),
	}
	if err := t.Base.Box.CloudClient().AddClockRecord(&req); err != nil {
		logger.Error().Err(err).Msg("could not add clock record in cloud")
		marshalled, err := json.Marshal(map[string]interface{}{
			"version":      CmdVersion,
			"cmd":          CmdTimeClockResp,
			"clock_status": int(timeClock.Action),
			"code":         -1,
		})
		if err != nil {
			logger.Error().Err(err).Msg("failed to marshal time clock error response, aborting")
			return
		}
		res, err := client.Post(cameraUrl, "application/json", bytes.NewReader(marshalled))
		if err != nil {
			logger.Error().Err(err).Str("camera_ip", camera.GetIP()).Str("camera_sn", camera.GetSN()).Msg("failed to send time clock error to scanner")
		}
		defer res.Body.Close()
	} else {
		marshalled, err := json.Marshal(map[string]interface{}{
			"version":      CmdVersion,
			"cmd":          CmdTimeClockResp,
			"clock_status": int(timeClock.Action),
			"code":         0,
		})
		if err != nil {
			logger.Error().Err(err).Msg("failed to marshal time clock error response, aborting")
			return
		}
		res, err := client.Post(cameraUrl, "application/json", bytes.NewReader(marshalled))
		if err != nil {
			logger.Error().Err(err).Str("camera_ip", camera.GetIP()).Str("camera_sn", camera.GetSN()).Msg("failed to send time clock success to scanner")
		}
		defer res.Body.Close()
	}
}
