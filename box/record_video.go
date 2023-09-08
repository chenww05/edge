package box

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/example/minibox/camera/base"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/scheduler"
	"github.com/example/minibox/utils"
	"github.com/example/turing-common/log"
)

const (
	MaxRetryTime                 = 3
	MaxRecordVideoDuration       = 3600
	RecordClipInterval     int64 = 100
)

var rvOnce sync.Once
var recordVideoProcess *RecordVideoProcess

type RecordVideoProcess struct {
	log       zerolog.Logger
	recording sync.Map
	device    Box
}

func GetRecordVideoProcess(device Box) *RecordVideoProcess {
	rvOnce.Do(func() {
		recordVideoProcess = &RecordVideoProcess{
			log:       log.Logger("recording"),
			recording: sync.Map{},
			device:    device,
		}
	})
	return recordVideoProcess
}

func (p *RecordVideoProcess) Start(req RecordVideoArg) error {
	_, loaded := p.recording.LoadOrStore(req.CameraID, req.StartedAt)
	if loaded {
		return errors.New("record")
	}
	baseCam, err := p.device.GetCamera(req.CameraID)
	if err != nil {
		return err
	}
	cam, ok := baseCam.(base.AICamera)
	if !ok {
		return ErrIncompatibleCamera
	}
	go p.recordAndArchiveVideo(cam, req)
	return nil
}

func (p *RecordVideoProcess) recordAndArchiveVideo(cam base.AICamera, req RecordVideoArg) error {
	nextTime := req.StartedAt
	wg := sync.WaitGroup{}
	taskID := req.TaskId
	videoPaths := make([]string, 0)
	defer func() {
		p.recording.Delete(cam.GetID())
		for _, videoPath := range videoPaths {
			if err := os.Remove(videoPath); err != nil {
				p.log.Warn().Err(err).Str("filename", videoPath).Msg("unable to delete temp file")
			}
		}
	}()

	resolution := string(utils.Normal)
	if req.Resolution == string(utils.HD) || req.Resolution == string(utils.SD) {
		resolution = req.Resolution
	}

	for nextTime < req.EndedAt {
		startedAt := nextTime
		endedAt := startedAt + RecordClipInterval
		if endedAt > req.EndedAt {
			endedAt = req.EndedAt
		}
		videoName := fmt.Sprintf("%s-%d-%d.mp4", taskID, startedAt, endedAt)

		cfg := p.device.GetConfig()
		endTime := endedAt + cfg.GetSecBeforeEvent()

		speed := scheduler.GetScheduler().GetDownloadSpeed()
		videoPath, _, _, err := cam.RecordVideo(resolution, startedAt, endTime, videoName, req.EnableAudio, speed)
		if err != nil {
			p.log.Error().Msgf("RecordVideo error: %s", err)
			nextTime = nextTime + RecordClipInterval
			continue
		}
		videoPaths = append(videoPaths, videoPath)
		wg.Add(1)
		go func() {
			var err error
			defer wg.Done()
			if err = p.uploadVideoToS3Retry(cam.GetID(), taskID, videoPath, startedAt, endedAt); err != nil {
				p.log.Error().Msgf("video file upload failed: %v", err)
				return
			}
		}()
		nextTime = nextTime + RecordClipInterval
	}
	wg.Wait()
	cli := p.device.CloudClient()
	if err := cli.UploadCameraVideoArchive(cam.GetID(), taskID,
		time.Unix(req.StartedAt, 0).Format(utils.CloudTimeLayout),
		time.Unix(req.EndedAt, 0).Format(utils.CloudTimeLayout)); err != nil {
		p.log.Error().Msgf("video archive failed: %v", err)
		return err
	}
	return nil
}

func (p *RecordVideoProcess) uploadVideoToS3Retry(cameraId int, taskID, videoPath string, startedAt, endedAt int64) error {
	for i := 0; i < MaxRetryTime; i++ {
		// upload video to s3
		p.log.Debug().Msgf("camera try upload video to s3, camera_id: %d, videoPath: %s, time: %d", cameraId, videoPath, i+1)
		err := p.uploadVideoToS3(cameraId, taskID, videoPath, startedAt, endedAt)
		if err == nil {
			break
		}
		time.Sleep(time.Second * time.Duration(i+1))
		continue
	}
	return nil
}

func (p *RecordVideoProcess) uploadVideoToS3(cameraId int, taskID, videoPath string, startedAt, endedAt int64) error {
	s3File, err := p.device.UploadS3ByTokenName(cameraId, videoPath, 0, 0, "mp4", TokenNameCameraVideo)
	if err != nil {
		p.log.Error().Msgf("video file upload s3 error: %v", err)
		return err
	}
	// upload s3Info to cloud
	var videos []cloud.MediaVideo
	videos = append(videos, cloud.MediaVideo{
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
	})
	cli := p.device.CloudClient()
	err = cli.UploadCameraVideo(cameraId, taskID, &cloud.Media{
		Videos: &videos,
	}, "")
	if err != nil {
		p.log.Error().Msgf("camera try upload media to cloud error: %v", err)
		return err
	}
	return nil
}
