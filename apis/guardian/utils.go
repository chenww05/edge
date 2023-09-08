package guardian

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/turingvideo/minibox/box"
	"github.com/turingvideo/minibox/camera/base"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/db"
	"github.com/turingvideo/minibox/utils"
)

func (g *GuardianAPI) getSnapshotFromEvent(event Event) string {
	snapshots, err := g.DB.GetSnapshotMediums(event.MediumIDs)
	if err != nil {
		return ""
	}
	if len(snapshots) > 0 {
		return snapshots[0].FilePath
	} else {
		return ""
	}
}

func (g *GuardianAPI) getVideoFromEvent(event Event) string {
	videos, err := g.DB.GetVideoMediums(event.MediumIDs)
	if err != nil {
		return ""
	}
	if len(videos) > 0 {
		return videos[0].FilePath
	} else {
		return ""
	}
}

func (g *GuardianAPI) handleUploadEvent(event Event) error {
	cam, err := g.Box.GetCamera(int(event.CameraID))
	if err != nil {
		g.logger.Error().Msgf("camera %d found in event is not a real camera in this box.", event.CameraID)
		return err
	}

	cfg := g.Box.GetConfig()
	saveEvent := cfg.GetEventSavedHours() > 0
	uploadCloud := !cfg.GetDisableCloud()
	uploadVideo := cam.GetUploadVideoEnabled()
	now := time.Now().UTC()

	var localEvent *db.Event
	var saved bool

	snapshotPath := g.getSnapshotFromEvent(event)
	videoPath := g.getVideoFromEvent(event)

	if saveEvent {
		localEvent, err = g.saveEventToDB(event, cam, now, snapshotPath, videoPath)
		if err != nil {
			return err
		}
		saved = true
	}
	if !uploadCloud {
		return nil
	}
	remoteID, err := g.uploadEventToCloud(event, now, snapshotPath)
	if err != nil {
		return err
	}
	if !saved || remoteID == "" {
		return nil
	}
	//	update event remote_id in db
	if err := g.DB.UpdateEvent(localEvent.ID, remoteID, int64(event.CameraID)); err != nil {
		g.logger.Error().Err(err).Uint("eventID", localEvent.ID)
		return err
	}

	if !uploadVideo {
		return nil
	}
	if s3File, err := g.uploadVideoToCloud(event, remoteID, videoPath); err != nil {
		var s3FileStr string
		if s3File != nil {
			buff, _ := json.Marshal(s3File)
			s3FileStr = string(buff)
		}
		if err = g.DB.UpdateEventVideo(localEvent.ID, videoPath, s3FileStr, true); err != nil {
			g.logger.Error().Str("eventType", event.Types).Str("remoteID", remoteID).Err(err).
				Msgf("failed to update event video info %v+", err)
		}
	}
	return nil
}

func (g *GuardianAPI) saveEventToDB(event Event, cam base.Camera, now time.Time, snapshotPath, videoPath string) (*db.Event, error) {
	imgContent, err := ioutil.ReadFile(snapshotPath)
	if err != nil {
		return nil, err
	}
	picInfo := db.FacePicture{
		Data:   imgContent,
		Height: -1,
		Width:  -1,
		Format: "JPEG",
	}
	g.logger.Info().Object("face_pic", &picInfo).Msg("saving picture to db")
	if err := g.DB.GetDBInstance().Create(&picInfo).Error; err != nil {
		g.logger.Error().Err(err).Msg("failed to save pic to db")
		return nil, err
	}

	localEvent := &db.Event{
		CameraSN:     cam.GetSN(),
		CameraID:     int64(cam.GetID()),
		StartedAt:    event.StartedAt.Time,
		EndedAt:      event.EndedAt.Time,
		CreatedAt:    now,
		UpdatedAt:    now,
		Type:         event.Types,
		PictureID:    picInfo.ID,
		PictureHash:  fmt.Sprintf("%x", md5.Sum(imgContent)),
		SnapshotPath: snapshotPath,
		VideoPath:    videoPath,
	}

	if err := g.DB.CreateEvent(localEvent); err != nil {
		g.logger.Error().Err(err).Msg("failed to save event to db")
		return nil, err
	}
	return localEvent, nil
}

func (g *GuardianAPI) uploadEventToCloud(event Event, now time.Time, snapshotPath string) (string, error) {
	//	upload snapshot to s3
	f, err := g.Box.UploadS3ByTokenName(int(event.CameraID), snapshotPath, -1, -1, "jpg", box.TokenNameCameraEvent)
	if err != nil {
		return "", err
	}
	remoteID, err := g.Box.UploadAICameraEvent(int(event.CameraID), f, event.StartedAt.Time, event.EndedAt.Time, now, event.Types, nil)
	if err == nil && remoteID != "" {
		if err := os.Remove(snapshotPath); err != nil {
			g.logger.Warn().Err(err).Str("filename", snapshotPath).Msg("unable to delete temp image file")
		}
	}
	return remoteID, err
}

func (g *GuardianAPI) uploadVideoToCloud(event Event, remoteID, videoPath string) (*utils.S3File, error) {
	var s3File *utils.S3File
	var finalErr error
	var err error
	for i := 0; i < 3; i++ {
		// upload video to s3
		finalErr = nil
		g.logger.Debug().Msgf("try upload video to s3, camera_id: %d, videopath: %s, time: %d", event.CameraID, videoPath, i+1)
		s3File, err = g.Box.UploadS3ByTokenName(int(event.CameraID), videoPath, -1, -1, "mp4", box.TokenNameCameraEvent)
		if err != nil {
			finalErr = fmt.Errorf("video file upload s3 error: %v", err)
			time.Sleep(time.Second * time.Duration(i+1))
			g.logger.Error().Err(finalErr).Msgf("upload media file to s3 error %d", event.CameraID)
			continue
		}

		// upload s3Info to cloud
		videos := []cloud.MediaVideo{{
			File: cloud.File{
				Meta: cloud.Meta{
					FileSize:    s3File.FileSize,
					Size:        []int{s3File.Height, s3File.Width},
					ContentType: "video/" + s3File.Format,
				},
				Key:    s3File.Key,
				Bucket: s3File.Bucket,
			},
			StartedAt: event.StartedAt.Time.Format(utils.CloudTimeLayout),
			EndedAt:   event.StartedAt.Time.Format(utils.CloudTimeLayout),
		}}
		g.logger.Debug().Msgf("try upload media: %s to event: %s, camera_id: %d, time: %d", s3File.Key, remoteID, event.CameraID, i+1)
		err = g.Box.UploadEventMedia(remoteID, &cloud.Media{
			Videos: &videos,
		})

		if err != nil {
			finalErr = fmt.Errorf("upload event media failed: %v", err)
			time.Sleep(time.Second * time.Duration(i+1))
			g.logger.Error().Err(finalErr).Msgf("upload event media to cloud error %d", event.CameraID)
			continue
		}
		break
	}

	// do clean when success
	if finalErr == nil {
		if err := os.Remove(videoPath); err != nil {
			g.logger.Warn().Err(err).Str("filename", videoPath).Msg("unable to delete temp image file")
		}
	}
	return s3File, finalErr
}
