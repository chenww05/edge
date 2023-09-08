package thermal_1

import (
	"errors"
)

type EventData struct {
	SN           string  `json:"deviceKey"`
	PersonID     string  `json:"personId"`
	Time         int     `json:"time"`
	Type         string  `json:"type"`
	Path         string  `json:"path"`
	ImgBase64    string  `json:"imgBase64"`
	Data         string  `json:"data"`
	IP           string  `json:"ip"`
	Temperature  float64 `json:"temperature"`
	Standard     float64 `json:"standard"`
	Abnormal     bool    `json:"temperatureState"`
	BusinessType string  `json:"business_type,omitempty"`
}

type AttendanceRecord struct {
	PersonID       int    `json:"employee_id"`
	Operation      string `json:"op_type"`
	OrganizationID int    `json:"organization_id"`
	EventID        int    `json:"event_id"`
}

func (a *AttendanceRecord) Check() error {
	if a.PersonID <= 0 {
		return errors.New("require employee_id")
	}
	if a.OrganizationID <= 0 {
		return errors.New("require organization_id")
	}
	// if a.EventID <= 0 {
	// 	return errors.New("require event_id")
	// }
	return nil
}

type RecognizeResult struct {
	ID             int    `json:"id"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Sex            string `json:"sex"`
	NO             string `json:"no"`
	Position       string `json:"position"`
	Department     string `json:"department"`
	Contract       string `json:"contract"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	OrganizationID int    `json:"org_id"`
	ImageBase64    string `json:"image"`
	EventID        int    `json:"event_id"`
}

type TimeClockInfo struct {
	TimeStamp  int    `json:"timestamp"`
	EmployeeID string `json:"id"`
	Action     int    `json:"clock_resp_status"`
	RfId       string `json:"rfid"`
	SN         string `json:"device_sn"`
}

type TimeClockAction string

const (
	ActionClockIn    TimeClockAction = "clock_in"
	ActionClockOut   TimeClockAction = "clock_out"
	ActionStartBreak TimeClockAction = "start_break"
	ActionEndBreak   TimeClockAction = "end_break"
)

type TimeClockStatus string

const (
	ClockStatusNew        TimeClockStatus = "new"
	ClockStatusInProgress TimeClockStatus = "in_progress"
	ClockStatusInBreak    TimeClockStatus = "in_a_break"
)

const (
	CmdVersion       string = "0.2"
	CmdFace          string = "face"
	CmdHeartBeat     string = "heart beat"
	CmdQuestionnaire string = "questionnaire"
	CmdCardReader    string = "ping card reader"
	CmdTimeClock     string = "time clock"
	CmdTimeClockResp string = "time clock response"
)
