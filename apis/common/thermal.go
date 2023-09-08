package common

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/turingvideo/minibox/apis/structs"
	"github.com/turingvideo/minibox/box"
	"github.com/turingvideo/minibox/camera/thermal_1"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/configs"
	"github.com/turingvideo/minibox/db"
	"github.com/turingvideo/minibox/printer"
	"github.com/turingvideo/minibox/utils"
)

type ThermalBaseAPI struct {
	Box    box.Box   `inject:"box"`
	DB     db.Client `inject:"db"`
	Logger zerolog.Logger
}

type ScanEvent struct {
	SN           string
	PrintData    *printer.PrintData
	FaceScan     *db.FaceScan
	Temperature  utils.Celsius
	Limit        utils.Celsius
	MetaScanData *structs.MetaScanData
}

func IsDataURI(data string) bool {
	return strings.HasPrefix(data, "data:")
}

// Will return empty string if IsDataURI is false
func GetDataURIData(uri string) string {
	index := strings.IndexRune(uri, ',')
	if index == -1 {
		return ""
	}

	return uri[index+1:]
}

// TODO maybe process base64 strings for the format?
func GetFormat(img string) string {
	return "jpg"
}

func (t *ThermalBaseAPI) UploadEventToCloud(
	cameraID int, temp, limit float64, face *db.FaceScan, meta *structs.MetaScanData, startedAt time.Time) (string, error) {
	var f *utils.S3File
	var err error
	if face != nil {
		f, err = t.UploadFileToS3(cameraID, face)
		if err != nil {
			return "", err
		}
	}
	return t.Box.UploadCameraEvent(cameraID, f, temp, limit, meta, startedAt)
}

func (t *ThermalBaseAPI) UploadFileToS3(cameraID int, face *db.FaceScan) (*utils.S3File, error) {
	filename, _, err := utils.SaveImage(t.Box.GetConfig().GetDataStoreDir(), face.ImgBase64)
	if err != nil {
		return nil, err
	}

	t.Logger.Debug().Str("filename", filename).Msg("Saved temp image file")
	defer func() {
		if err := os.Remove(filename); err != nil {
			t.Logger.Warn().Err(err).Str("filename", filename).Msg("unable to delete temp image file")
		}
	}()
	return t.Box.UploadS3ByTokenName(cameraID, filename, face.Rect.Height, face.Rect.Width, "jpg", box.TokenNameCameraEvent)
}

func (t *ThermalBaseAPI) skipEvent(config configs.Config, req ScanEvent) bool {
	cfg := config.GetVisitorConfig()

	if req.MetaScanData == nil {
		return false
	}
	visitor := req.MetaScanData.PersonRole == thermal_1.PersonRoleVisitor
	if !visitor {
		return false
	}
	if cfg.DisableFailedQuestionnaire && req.MetaScanData.HasQuestionnaire && !req.MetaScanData.QuestionnaireResult {
		return true
	}
	if cfg.DisableFailedTemperature && req.Temperature > req.Limit {
		return true
	}
	return false
}

func (t *ThermalBaseAPI) HandleUploadEvent(req ScanEvent) error {
	logger := t.Logger.With().Str("function", "common HandleUploadEvent").Str("camera_sn", req.SN).Logger()

	baseCam, err := t.Box.GetCameraBySN(req.SN)
	if err != nil {
		logger.Error().Err(err).Msg("camera not found, aborting")
		return err
	}

	baseCam.Heartbeat(baseCam.GetIP(), "")
	cfg := t.Box.GetConfig()

	if t.skipEvent(cfg, req) {
		logger.Info().Interface("config", cfg).Interface("req", req).Msg("skipping event due to config")
		return nil
	}
	cameraID := baseCam.GetID()
	eventChan := make(chan uint, 1)

	saveEvent := cfg.GetEventSavedHours() > 0
	uploadCloud := cameraID > 0 && !cfg.GetDisableCloud()

	go func() {
		if err := t.Box.GetPrintStrategy().Print(context.Background(), req.PrintData); err != nil {
			logger.Error().Err(err).Msg("failed to print")
		}
	}()

	if req.FaceScan != nil {
		isF := cfg.GetTemperatureUnit() == configs.FUnit
		go baseCam.SendEventToWSConn(isF, req.FaceScan.ImgBase64, req.Temperature, req.Limit)
	}

	if saveEvent {
		go func() {
			eventID, err := t.DB.SaveEventToDB(req.SN, cameraID, req.Temperature > req.Limit, req.FaceScan, req.MetaScanData)
			if err != nil {
				logger.Error().Err(err).Msg("failed to save event to db")
				close(eventChan)
				return
			}
			eventChan <- eventID
		}()
	} else {
		close(eventChan)
	}

	if uploadCloud {
		go func() {
			now := time.Now().UTC()
			remoteID, err := t.UploadEventToCloud(cameraID, float64(req.Temperature), float64(req.Limit), req.FaceScan, req.MetaScanData, now)
			if err != nil {
				logger.Error().Err(err).Msg("failed to upload event to cloud")
			}

			var cloudErr cloud.Err
			eventID, ok := <-eventChan
			if ok && remoteID != "" {
				err = t.DB.UpdateEvent(eventID, remoteID, int64(cameraID))
				if err != nil {
					logger.Error().Err(err).Uint("event_id", eventID).Str("remote_id", remoteID).Msg("failed to update event")
				}
			} else if remoteID == "" && !saveEvent && !errors.As(err, &cloudErr) {
				_, err := t.DB.SaveEventToDB(req.SN, cameraID, req.Temperature > req.Limit, req.FaceScan, req.MetaScanData)
				if err != nil {
					logger.Error().Err(err).Msg("failed to save event to db, LOST EVENT")
				}
			}
		}()
	} else if cameraID == 0 {
		logger.Error().Msg("Camera has no id")
		if !saveEvent {
			go func() {
				_, err := t.DB.SaveEventToDB(req.SN, cameraID, req.Temperature > req.Limit, req.FaceScan, req.MetaScanData)
				if err != nil {
					t.Logger.Error().Err(err).Msg("failed to save event to db, LOST EVENT")
				}
			}()
		}
	}

	return nil
}
