package common

import (
	"github.com/rs/zerolog"
	"github.com/example/minibox/box"
	"github.com/example/minibox/db"
)

type PplBaseAPI struct {
	Box    box.Box   `inject:"box"`
	DB     db.Client `inject:"db"`
	Logger zerolog.Logger
}
