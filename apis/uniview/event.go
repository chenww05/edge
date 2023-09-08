package uniview

import (
	"time"

	"github.com/turingvideo/minibox/box"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/utils"
)

const (
	maxRetry  = 3
	videoType = "mp4"
)

func (u UniviewAPI) uploadEventMediaToCloud(cameraID int, eventID, videoPath string, startedAt, endedAt int64) (*utils.S3File, error) {
	var err error
	var s3File *utils.S3File
	for i := 0; i < maxRetry; i++ {
		s3File, err = u.Box.UploadS3ByTokenName(cameraID, videoPath, 0, 0, videoType, box.TokenNameCameraEvent)
		if err != nil {
			continue
		} else {
			break
		}
	}
	if err != nil {
		return s3File, err
	}

	videos := []cloud.MediaVideo{
		{
			File: cloud.File{
				Meta: cloud.Meta{
					FileSize:    s3File.FileSize,
					Size:        []int{s3File.Height, s3File.Width},
					ContentType: "video/" + s3File.Format,
				},
				Key:    s3File.Key,
				Bucket: s3File.Bucket,
			},
			StartedAt: time.Unix(startedAt, 0).Format(utils.CloudTimeLayout),
			EndedAt:   time.Unix(endedAt, 0).Format(utils.CloudTimeLayout),
		},
	}
	if err = u.Box.UploadEventMedia(eventID, &cloud.Media{
		Videos: &videos,
	}); err != nil {
		return s3File, err
	}
	u.Logger.Debug().Msgf("upload event ID = %s successfully", eventID)
	return s3File, nil
}
