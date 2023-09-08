package common

import (
	"github.com/rs/zerolog"
	"github.com/turingvideo/minibox/box"
	"github.com/turingvideo/minibox/db"
)

type PplBaseAPI struct {
	Box    box.Box   `inject:"box"`
	DB     db.Client `inject:"db"`
	Logger zerolog.Logger
}
