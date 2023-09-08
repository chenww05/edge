package thermal_1

import (
	"math"
	"sync"
	"time"

	"github.com/jinzhu/copier"
	"github.com/turingvideo/minibox/apis/common"
	"github.com/turingvideo/minibox/apis/structs"
	"github.com/turingvideo/minibox/camera/thermal_1"
	"github.com/turingvideo/minibox/db"
	"github.com/turingvideo/minibox/printer"
	"github.com/turingvideo/minibox/rfid"
	"github.com/turingvideo/minibox/utils"
)

func NewFaceCacheSet() map[string]*faceCache {
	return make(map[string]*faceCache)
}

func NewFaceCache(base common.ThermalBaseAPI) *faceCache {
	fc := &faceCache{
		counter:        0,
		mux:            sync.Mutex{},
		data:           nil,
		ThermalBaseAPI: base,
	}
	return fc
}

// TODO(zhaowentao) refactor it
type faceCache struct {
	// cache is empty if counter equals zero
	counter int
	mux     sync.Mutex
	data    *thermal_1.FaceInfo
	common.ThermalBaseAPI
}

func (c *faceCache) Add(data *thermal_1.FaceInfo) {
	c.Logger.Debug().Interface("face_info", data).Msg("Add data to gen 1 cache")
	c.mux.Lock()
	defer func() {
		c.counter++
		c.mux.Unlock()
	}()

	cfg := c.Box.GetConfig()

	// Read RfId data
	if cfg.ShouldScanRfid() {
		c.Logger.Info().Msg("reading rfid data from redis")
		rfId, err := rfid.GetRfid()
		if err != nil {
			c.Logger.Error().Err(err).Msg("failed to get rfId")
		} else if data.RfId != "" {
			c.Logger.Debug().Msgf("got rfId: %s", rfId)
			data.RfId = rfId
		}
	}

	if c.counter == 0 {
		c.data = data
		secs := cfg.GetScanPeriodSecs()
		go c.waitAndCreateEvent(secs)
	}

	if len(data.RfId) > 0 {
		c.data.RfId = data.RfId
	}

	if data.Person != nil {
		if c.data.Person == nil {
			c.data.Person = data.Person
		}

		// if has temperature, update temperature
		newT := data.Person.Temperature
		oldT := c.data.Person.Temperature
		hasTemperature := newT != 0
		if hasTemperature {
			baseCam, err := c.Box.GetCameraBySN(data.SN)
			if err != nil {
				c.Logger.Error().Msgf("not found camera: %s", err)
				return
			}
			cam, ok := baseCam.(TemperatureQuery)
			if !ok {
				c.Logger.Error().Msgf("not supported camera info %v", baseCam)
				return
			}
			tc, err := cam.GetTemperatureConfig()
			if err != nil {
				c.Logger.Error().Msgf("failed to get camera temperature config")
				return
			}

			min, max := tc.Min, tc.Max
			if tc.FahrenheitUnit {
				min = utils.FtoC(min)
				max = utils.FtoC(max)
			}

			if !(min <= oldT && oldT <= max) { // fix initial temp out of range
				c.Logger.Error().Float64("oldT", oldT).
					Float64("min", min).
					Float64("max", max).
					Msg("Initial temperature out of range")

				oldT = 0
			}

			if min <= newT && newT <= max {
				if oldT == 0 {
					c.data.Person.Temperature = newT
				} else {
					c.data.Person.Temperature = (newT + oldT) / 2
				}
			}
		}
	}

	// if has QR code, update QR code
	if data.QrCode != nil {
		c.data.QrCode = data.QrCode
	}

	// if has face image, update face image
	if data.Picture != nil {
		c.data.Picture = data.Picture
	}
}

func (c *faceCache) GetAndClean() thermal_1.FaceInfo {
	c.mux.Lock()
	defer c.mux.Unlock()
	val := thermal_1.FaceInfo{}
	copier.Copy(&val, c.data)
	c.data = nil
	c.counter = 0

	return val
}

func toPrintData(tc thermal_1.TemperatureConfig, info thermal_1.FaceInfo, meta *structs.MetaScanData) *printer.PrintData {
	temp := utils.Celsius(info.Person.Temperature)
	var (
		overTemp bool
		photo    *string
	)

	if tc.FahrenheitUnit {
		overTemp = temp > utils.Fahrenheit(tc.Limit).ToC()
	} else {
		overTemp = temp > utils.Celsius(tc.Limit)
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

func getMetaData(temp, limit float64, qrCode *thermal_1.QrCode, rfId string) *structs.MetaScanData {
	data := structs.MetaScanData{
		Temperature: temp,
		Abnormal:    limit,
		RfId:        rfId,
	}

	if qrCode != nil {
		data.QrCode = &structs.QrCode{
			QrType: qrCode.QrType,
			QrData: qrCode.QrData,
		}
	}

	return &data
}

// The device may support recognition/detection of one or more types of data
// such as close-up face images, body temperature, QR code information, etc, in a scanning period.
// Note: Some devices may upload all three types of data.
// We process as follows:
// If no temperature detected, there is no need to get temperature related configuration from device.
// Otherwise, we need to get the temperature related configuration to check
// whether the body temperature is abnormal, etc.
// But we konow,get device configuration may failed, if failed, we need give up upload.
// If there is neither face image nor temperature, we also need give up upload.
// If one exists, event needs to be created and reported.
func (c *faceCache) waitAndCreateEvent(secs int) {
	c.Logger.Info().Int("sleep_seconds", secs).Msg("start waitAndCreateEvent")

	time.Sleep(time.Second * time.Duration(secs))

	req := c.GetAndClean()
	c.Logger.Debug().Interface("face_info", req).Msg("retrieved face info from cache")

	baseCam, err := c.Box.GetCameraBySN(req.SN)
	if err != nil {
		c.Logger.Error().Err(err).Str("camera_sn", req.SN).Msg("camera not found in memory, aborting")
		return
	}

	req.Person.Temperature = math.Ceil(req.Person.Temperature*100) / 100
	t := utils.Celsius(req.Person.Temperature)

	hasFaceImage, hasTemperature := req.Picture != nil, t > 0
	if !hasFaceImage && !hasTemperature {
		c.Logger.Info().Msg("no face image, no temperature, aborting upload")
		return
	}

	cam, ok := baseCam.(TemperatureQuery)
	if !ok {
		c.Logger.Error().Str("brand", string(baseCam.GetBrand())).Msg("wrong camera type")
		return
	}

	tc, err := cam.GetTemperatureConfig()
	if err != nil {
		c.Logger.Error().Err(err).Msg("could not get temperature settings from scanner, aborting")
		return
	}

	var limit utils.Celsius
	if tc.FahrenheitUnit {
		limit = utils.Fahrenheit(tc.Limit).ToC()
		c.Logger.Debug().Float64("tc_limit", tc.Limit).Float64("limit", float64(limit)).Msg("converting temperature config limit from farenheight to celsius")
	} else {
		limit = utils.Celsius(tc.Limit)
	}

	meta := getMetaData(req.Person.Temperature, float64(limit), req.QrCode, req.RfId)
	// Gen 1 has no questionnaire result
	//meta.QuestionnaireResult = true
	event := common.ScanEvent{
		SN:           req.SN,
		PrintData:    toPrintData(*tc, req, meta),
		MetaScanData: meta,
		Temperature:  utils.Celsius(req.Person.Temperature),
		Limit:        limit,
	}

	if req.Picture != nil {
		event.FaceScan = &db.FaceScan{
			ImgBase64: req.Picture.Data,
			Format:    req.Picture.Format,
			Rect: db.FaceRect{
				X:      req.Picture.FaceX,
				Y:      req.Picture.FaceY,
				Height: req.Picture.FaceHeight,
				Width:  req.Picture.FaceWidth,
			},
		}
	}

	if err := c.HandleUploadEvent(event); err != nil {
		c.Logger.Error().Err(err).Msg("Common upload event handler error")
	}
}
