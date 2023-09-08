package structs

import (
	"github.com/rs/zerolog"

	"github.com/example/minibox/camera/thermal_1"
)

type QrCode struct {
	QrType string `json:"qrtype"`
	QrData string `json:"qrdata"`
}

type HumanCountInfo struct {
	Enter                 int     `json:"in_num"`
	Exit                  int     `json:"out_num"`
	Method                string  `json:"method"`
	BaseNum               int     `json:"initial_num"`
	NotificationThreshold int     `json:"notification_threshold"`
	Threshold             float64 `json:"threshold"`
	Capacity              int     `json:"capacity"`
}

type Rectangle struct {
	X      float64 `json:"x"` // top left corner x
	Y      float64 `json:"y"` // top left corner y
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type ObjectInfo struct {
	Confidence int       `json:"confidence"`
	BBox       Rectangle `json:"b_box"`
}

type PolygonInfo struct {
	Points       []Point `json:"points"`
	EnabledTypes []int   `json:"-"` // 0:Motor Vehicle,1:Non-Motor Vehicle,2:Pedestrian
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type OSDTextArea struct {
	Points []Point `json:"points"`
}

type MetaScanData struct {
	QrCode               *QrCode              `json:"qrcode,omitempty"`
	RfId                 string               `json:"rfid,omitempty"`
	Temperature          float64              `json:"temperature,omitempty"`
	Abnormal             float64              `json:"abnormal,omitempty"`
	QuestionnaireResult  bool                 `json:"questionnaire_result"`
	HasQuestionnaire     bool                 `json:"has_questionnaire"`
	Company              string               `json:"company,omitempty"`
	PersonName           string               `json:"person_name,omitempty"`
	PersonId             string               `json:"person_id,omitempty"`
	PersonRole           thermal_1.PersonRole `json:"person_role"`
	PhoneNumber          string               `json:"phone_number,omitempty"`
	VisitingWho          string               `json:"visiting_who,omitempty"`
	Email                string               `json:"email,omitempty"`
	Site                 string               `json:"site,omitempty"`
	RetryCount           uint                 `json:"retry_count,omitempty"`
	HumanCount           *HumanCountInfo      `json:"human_count"`
	PolygonInfos         []PolygonInfo        `json:"polygon_infos,omitempty"`
	Objects              []ObjectInfo         `json:"objects,omitempty"`
	SnapshotOSDTextAreas []OSDTextArea        `json:"snapshot_osd_text_areas,omitempty"`
	VideoOSDTextAreas    []OSDTextArea        `json:"video_osd_text_areas,omitempty"`
}

type PrintingConditions struct {
	PersonRole          thermal_1.PersonRole `json:"person_role"`
	OverTemp            bool                 `json:"over_temp"`
	QuestionnaireResult bool                 `json:"questionnaire_result"`
}

func (s *QrCode) MarshalZerologObject(e *zerolog.Event) {
	if s == nil {
		return
	}

	e.Str("qrtype", s.QrType).Str("qrdata", s.QrData)
}

func (s *MetaScanData) MarshalZerologObject(e *zerolog.Event) {
	if s == nil {
		return
	}

	e.Str("rfid", s.RfId).
		Str("company", s.Company).
		Str("person_name", s.PersonName).
		Str("person_id", s.PersonId).
		Float64("temperature", s.Temperature).
		Float64("abnormal", s.Abnormal).
		Bool("questionnaire_result", s.QuestionnaireResult).
		Bool("has_questionnaire", s.HasQuestionnaire).
		Object("qr_code", s.QrCode).
		Int("person_role", int(s.PersonRole)).
		Str("visiting_who", s.VisitingWho).
		Str("phone_number", s.PhoneNumber).
		Str("email", s.Email).
		Str("site", s.Site)
}
