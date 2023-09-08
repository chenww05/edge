package sunell

import (
	"time"

	"github.com/example/minibox/apis/structs"
	"github.com/example/minibox/db"
	"github.com/example/minibox/printer"
	"github.com/example/minibox/utils"
)

const (
	DeviceID         = "DeviceID"
	TargetId         = "TargeId"
	AlarmType        = "AlarmType"
	AlarmTime        = "AlarmTime"
	AIPictureDataLen = "AIPictureDataLen"
	AIPictureData    = "AIPictureData"
	AIPictureType    = "AIPictureType"

	FaceX           = "FaceX"
	FaceY           = "FaceY"
	FaceWidth       = "FaceWidth"
	FaceHeight      = "FaceHeight"
	TemperatureUnit = "TemperatureUnit"
	FaceTemperature = "FaceTemperature"

	CarColor = "CarColor"
	CarMode  = "CarMode"

	AlarmTypeFace    = "0"
	AlarmTypeBody    = "2"
	AlarmTypeVehicle = "4"

	PictureTypePart    = "1"
	PictureTypeOverall = "0"

	DefaultImgFormat = "jpg"

	CameraBrandMissMatchErr = "camera brand miss match"

	DefaultCameraVerson = "1.0"
)

type SunellEvent struct {
	DeviceID  string    `json:"device_id"`
	TargetID  string    `json:"target_id"`
	TimeStamp int       `json:"timestamp"`
	Time      time.Time `json:"time"`
	Type      string    `json:"type"`
	ImgBase64 string    `json:"imgBase64"`
	ImgFormat string    `json:"imgFormat"`
	// face
	FaceX      int `json:"face_x"`
	FaceY      int `json:"face_y"`
	FaceWidth  int `json:"face_width"`
	FaceHeight int `json:"face_height"`
	// vehicle
	CarColor int `json:"car_color"`
	CarMode  int `json:"car_mode"`
}

type ScanEvent struct {
	SN           string
	PrintData    *printer.PrintData
	FaceScan     *db.FaceScan
	Temperature  utils.Celsius
	Limit        utils.Celsius
	MetaScanData *structs.MetaScanData
	StartedAt    time.Time
	EndedAt      time.Time
}

type HeartbeatInfo struct {
	MAC    string `json:"mac"`
	IPV4   string `json:"ipv4"`
	Vendor string `json:"vendor"`
	Online bool   `json:"online"`
}
