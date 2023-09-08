package uniview

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/example/turing-common/log"

	"github.com/example/minibox/camera/uniview"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/utils"
)

const (
	MotionAlarmOff   = "MotionAlarmOff"
	MotionAlarmOn    = "MotionAlarmOn"
	MinVideoDuration = 2
	MinPartDuration  = 0.3
)

var (
	ErrNoMotionEvent = errors.New("no motion event")
	ErrNoRemoteID    = errors.New("no remote id")
	ErrNoVideoPath   = errors.New("no video path")
	ErrNoImages      = errors.New("no images")
)

type MotionProcess interface {
	isMotionStart(cameraID int) bool
	pushMotion(event *motionEvent)
	popAllMotions(cameraID int) motionEvents
	uploadMotionStartEvent(event *motionEvent, u *UniviewAPI, univCam *uniview.BaseUniviewCamera, imageInfoList []ImageInfo) error
	uploadMotionEventsVideo(events motionEvents, u *UniviewAPI, univCam *uniview.BaseUniviewCamera) error
}

type motionProcess struct {
	mu                sync.RWMutex
	cameraMotionGroup map[int]*motionGroup
	logger            zerolog.Logger
}

func newMotionProcess() *motionProcess {
	return &motionProcess{
		mu:                sync.RWMutex{},
		cameraMotionGroup: make(map[int]*motionGroup),
		logger:            log.Logger("motion-process"),
	}
}

type motionGroup struct {
	events motionEvents
}

type motionEvent struct {
	remoteID    string
	motionType  string
	cameraID    int
	timestamp   int64
	receiveTime time.Time
	imgBase64   string
}

type motionEvents []*motionEvent

func (s motionEvents) Len() int           { return len(s) }
func (s motionEvents) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s motionEvents) Less(i, j int) bool { return s[i].timestamp < s[j].timestamp }

func (p *motionProcess) isMotionStart(cameraID int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	group, exist := p.cameraMotionGroup[cameraID]
	if !exist || group == nil || len(group.events) == 0 {
		return true
	}
	return false
}

// pushMotion when revive motion_start,return group length
func (p *motionProcess) pushMotion(event *motionEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	group, exist := p.cameraMotionGroup[event.cameraID]
	if !exist {
		group = &motionGroup{events: motionEvents{}}
		p.cameraMotionGroup[event.cameraID] = group
	}
	group.events = append(group.events, event)
}

// popAllMotions pops all motions when revive motion_end
func (p *motionProcess) popAllMotions(cameraID int) motionEvents {
	p.mu.Lock()
	defer p.mu.Unlock()

	group, exist := p.cameraMotionGroup[cameraID]
	if !exist || len(group.events) == 0 {
		return nil
	}
	// clean motion
	defer func() {
		group = nil
		delete(p.cameraMotionGroup, cameraID)
	}()

	return group.events
}

func getPartDuration(number int) float64 {
	partDuration := MinVideoDuration / float64(number)
	if partDuration < MinPartDuration {
		partDuration = MinPartDuration
	}
	return partDuration
}

func (p *motionProcess) genProtocolFile(events motionEvents, storeDir string) (string, []string, error) {
	imgPaths := make([]string, 0)
	file, err := ioutil.TempFile(storeDir, fmt.Sprintf("%d*.txt", time.Now().Unix()))
	if err != nil {
		return "", nil, err
	}
	defer file.Close()
	filename := file.Name()
	partDuration := getPartDuration(len(events))
	for _, event := range events {
		imgPath, _, err := utils.SaveImage(storeDir, event.imgBase64)
		if err != nil {
			p.logger.Err(err).Msg("save image error")
			continue
		}
		imgPaths = append(imgPaths, imgPath)
		content := []byte(fmt.Sprintf("file './%s'\nduration %.2f\n", path.Base(imgPath), partDuration))
		if n, err := file.Write(content); err != nil {
			p.logger.Err(err).Msgf("write file error, n is %d", n)
			continue
		}
		p.logger.Info().Msgf("event %d, %d, %s, %.2f", event.cameraID, event.timestamp, filename, partDuration)
	}
	if len(imgPaths) == 0 {
		return filename, nil, ErrNoImages
	}
	lastImg := imgPaths[len(imgPaths)-1]
	content := []byte(fmt.Sprintf("file './%s'\n", path.Base(lastImg)))
	if n, err := file.Write(content); err != nil {
		p.logger.Err(err).Msgf("write file error, n is %d", n)
		return filename, nil, err
	}
	return filename, imgPaths, nil
}

// compressSnapshots use ffmpeg
func (p *motionProcess) compressSnapshots(events motionEvents, storeDir string) (string, error) {
	filename, imgPaths, err := p.genProtocolFile(events, storeDir)
	deleteFile := func(filepath string) {
		if len(filepath) == 0 {
			return
		}
		if err = os.Remove(filepath); err != nil {
			p.logger.Warn().Err(err).Str("filename", filepath).Msg("unable to delete file")
		}
	}
	defer func() {
		deleteFile(filename)
		for _, imgPath := range imgPaths {
			deleteFile(imgPath)
		}
	}()
	if err != nil {
		return "", err
	}
	// compress video us ffmpeg
	videoPath := strings.ReplaceAll(filename, "txt", "mp4")
	params := utils.GetCompressVideoCommandArgs(filename, videoPath)
	// This command takes about 3s
	cmd := exec.Command("ffmpeg", params...)
	var outLog bytes.Buffer
	var errLog bytes.Buffer
	cmd.Stdout = &outLog
	cmd.Stderr = &errLog
	err = cmd.Run()
	if err != nil {
		p.logger.Error().Msg(fmt.Sprintf("ffmpeg command error: %s", errLog.String()))
		return videoPath, err
	}
	p.logger.Info().Msgf("compress video success %s", videoPath)
	return videoPath, nil
}

type mergedEvent struct {
	event     *motionEvent
	startTime int64
	endTime   int64
}

// mergeEvents when receive motion_end
func (p *motionProcess) mergeEvents(events motionEvents, storeDir string) (*mergedEvent, string, error) {
	if len(events) == 0 {
		return nil, "", ErrNoMotionEvent
	}
	sort.Sort(events)
	videoPath, err := p.compressSnapshots(events, storeDir)
	if err != nil {
		return nil, videoPath, err
	}
	p.logger.Debug().Msgf("video path %s", videoPath)
	startTime := events[0].timestamp
	endTime := events[len(events)-1].timestamp
	return &mergedEvent{event: events[0], startTime: startTime, endTime: endTime}, videoPath, nil
}

func (p *motionProcess) uploadMotionStartEvent(event *motionEvent, u *UniviewAPI, univCam *uniview.BaseUniviewCamera, imageInfoList []ImageInfo) error {
	if !cloud.HasEventType(univCam.ID, cloud.MotionStart) {
		msg := fmt.Sprintf("you don't have license: %s on this camera: %d", cloud.MotionStart, univCam.ID)
		u.Logger.Warn().Msg(msg)
		return errors.New(msg)
	}

	eventTime := event.timestamp
	cfg := u.Box.GetConfig()
	if (eventTime - univCam.GetLastEventDetectTime()) < cfg.GetEventIntervalSecs() {
		u.Logger.Warn().Msgf("Filter this event,interval secs is %d(s), this trigger time is %d, previous event "+
			"trigger time is %d", cfg.GetEventIntervalSecs(), eventTime, univCam.GetLastEventDetectTime())
		return nil
	}
	univCam.SetLastEventDetectTime(eventTime)
	saveEvent := cfg.GetEventSavedHours() > 0
	uploadCloud := !cfg.GetDisableCloud()
	videoDuration := cfg.GetVideoClipDuration()
	uploadVideo := univCam.GetUploadVideoEnabled()

	remoteID, err := u.handlerOtherEvent(univCam, eventTime, imageInfoList, saveEvent, uploadCloud, uploadVideo, event.receiveTime, videoDuration)
	if err != nil {
		u.Logger.Err(err).Msgf("processMotionEvent error cameraID %d", univCam.GetID())
		return err
	}
	if len(remoteID) == 0 {
		return ErrNoRemoteID
	}
	event.remoteID = remoteID
	return nil
}

func (p *motionProcess) uploadMotionEventsVideo(events motionEvents, u *UniviewAPI, univCam *uniview.BaseUniviewCamera) error {
	if !cloud.HasEventType(univCam.ID, cloud.MotionStart) {
		msg := fmt.Sprintf("you don't have license: %s on this camera: %d", cloud.MotionStart, univCam.ID)
		u.Logger.Warn().Msg(msg)
		return errors.New(msg)
	}

	ret, videoPath, err := p.mergeEvents(events, u.Box.GetConfig().GetDataStoreDir())
	defer func() {
		if len(videoPath) > 0 {
			if err = os.Remove(videoPath); err != nil {
				u.Logger.Warn().Err(err).Str("filename", videoPath).Msg("unable to delete temp image file")
			}
		}
	}()
	if err != nil {
		return err
	}
	if videoPath == "" {
		return ErrNoVideoPath
	}
	// upload media video to s3
	if s3File, err := u.uploadEventMediaToCloud(univCam.GetID(), ret.event.remoteID, videoPath, ret.startTime, ret.endTime); err != nil {
		u.Logger.Error().Err(err).Msgf("failed to upload event media to cloud %v", s3File)
		return err
	}
	return nil
}
