package guardian

import (
	"time"
)

type Medium struct {
	ID uint `json:"id"`
}

type Event struct {
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	StartedAt GuardianTime `json:"started_at"`
	EndedAt   GuardianTime `json:"ended_at"`
	RemoteID  uint         `json:"remote_id"`
	CameraID  uint         `json:"camera_id"`
	Mediums   []Medium     `json:"mediums"`
	MediumIDs []uint       `json:"medium_ids"`
	Types     string       `json:"types"`
	TypesBits string       `json:"types_bits"`
	ObjTypes  interface{}  `json:"obj_types"`
	Remark    string       `json:"remark"`
	Metadata  interface{}  `json:"metadata"`
}

func (e *Event) Check() {
	// get snapshot first
	for _, medium := range e.Mediums {
		e.MediumIDs = append(e.MediumIDs, medium.ID)
	}

}
