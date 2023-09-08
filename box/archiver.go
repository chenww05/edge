package box

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/example/turing-common/log"
	"github.com/example/turing-common/metrics"
	"github.com/example/turing-common/model"

	"github.com/example/streamer"

	"github.com/example/minibox/camera/base"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/configs"
	"github.com/example/minibox/db"
	"github.com/example/minibox/utils"
)

var (
	atr  *ArchiveTaskRunner
	once sync.Once
)

const (
	archiveSchedulerHeartbeat = 20 * time.Second
	maxRetry                  = 3

	taskStatusOn  = "on"
	taskStatusOff = "off"

	initialTaskSize       = 16
	maxStopNotifyChanSize = 16
	maxOutputChanSize     = 256

	syncSettingsInterval = time.Hour * 1
	recoverInterval      = time.Hour * 1
	maxRecoverDurations  = time.Hour * 24

	day = 24 * 60 * 60

	minRecoverySeconds = 2
	disableAudio       = true
	taskTypeRecovery   = "recovery"

	DiskUsagePath = "/"
)

var isDiskFull bool
var isDiskFullNotified bool

type ArchiveTaskRunner struct {
	device                                Box
	db                                    db.Client
	logger                                zerolog.Logger
	lock                                  sync.Mutex
	tasks                                 map[string]*model.CloudStorageSetting
	outputChan                            chan *streamer.OutputFile
	stopNotify                            chan streamer.StopNotify
	limiter                               utils.Limiter
	cloudStoragePauseDiskUsage            int
	cloudStorageResumeDiskUsage           int
	cloudStorageTaskTypeRecoveryMaxNumber int
}

func NewArchiveTaskRunner(device Box, d db.Client, config configs.CloudStorageConfig) *ArchiveTaskRunner {
	atr = &ArchiveTaskRunner{
		device:                                device,
		db:                                    d,
		logger:                                log.Logger("cloud storage"),
		tasks:                                 make(map[string]*model.CloudStorageSetting, initialTaskSize),
		outputChan:                            make(chan *streamer.OutputFile, maxOutputChanSize),
		stopNotify:                            make(chan streamer.StopNotify, maxStopNotifyChanSize),
		limiter:                               make(chan struct{}, config.MaxConcurrentUploadSize),
		cloudStoragePauseDiskUsage:            config.CloudStoragePauseDiskUsage,
		cloudStorageResumeDiskUsage:           config.CloudStorageResumeDiskUsage,
		cloudStorageTaskTypeRecoveryMaxNumber: config.CloudStorageTaskTypeRecoveryMaxNumber,
	}
	once.Do(func() {
		go atr.handleNewSegment()
		go atr.uploadSegments()
		go atr.recoverSegments()
		go atr.cleanupRecords()
		go atr.metric()
	})
	return atr
}

func GetArchiveTaskRunner() *ArchiveTaskRunner {
	return atr
}

func (a *ArchiveTaskRunner) DumpTasks() []map[string]string {
	ret := []map[string]string{}
	for k, task := range a.tasks {
		taskDump := make(map[string]string)
		taskDump["key"] = k
		taskDump["id"] = fmt.Sprintf("%d", task.Id)
		taskDump["camId"] = fmt.Sprintf("%d", task.CameraId)
		taskDump["status"] = task.Status
		taskDump["resolution"] = task.Resolution
		taskDump["videoExpire"] = fmt.Sprintf("%d", task.VideoExpire)
		taskDump["isLive"] = fmt.Sprintf("%+v", task.IsLiveMode)
		taskDump["createdAt"] = fmt.Sprintf("%d", task.CreatedAt.Unix())
		taskDump["createdTime"] = fmt.Sprintf("%d", task.CreatedTime)
		taskDump["type"] = task.TaskType
		taskDump["streamStart"] = fmt.Sprintf("%d", task.StreamStartTime)
		taskDump["streamEnd"] = fmt.Sprintf("%d", task.StreamEndTime)
		taskDump["runningStatus"] = fmt.Sprintf("%d", task.RunningStatus)
		taskDump["streamUrl"] = task.StreamUrl
		ret = append(ret, taskDump)
	}
	return ret
}

func (a *ArchiveTaskRunner) handleNewSegment() {
	for {
		select {
		case msg := <-a.outputChan:
			// avoid empty file or record, it will impact findMissingRanges.
			if msg.EndTime-msg.StartTime <= 0 {
				os.Remove(msg.TargetFilePath)
				break
			}
			av := &db.ArchiveVideo{
				CameraID:  msg.Task.CameraId,
				TaskId:    msg.Task.Id,
				TaskType:  a.getTaskKey(msg.Task),
				StartTime: msg.StartTime,
				EndTime:   msg.EndTime,
				FilePath:  msg.TargetFilePath,
			}
			a.logger.Debug().Msgf("camera id:%d, task id:%s, is live:%v generate %s from %d to %d",
				av.CameraID, av.TaskType, msg.Task.IsLiveMode, msg.TargetFilePath, msg.StartTime, msg.EndTime)
			if err := a.db.CreateArchiveVideo(av); err != nil {
				a.logger.Err(err).Msgf("camera id:%d, task id:%s failed to create archive video", av.CameraID, av.TaskType)
			}
		case result := <-a.stopNotify:
			css, ok := result.Data.(*model.CloudStorageSetting)
			if !ok {
				break
			}
			if css.TaskType == taskTypeRecovery {
				a.lock.Lock()
				css.Cancel()
				delete(a.tasks, a.getTaskKey(css))
				a.lock.Unlock()
			} else {
				// When live recording failed, try to refresh stream url here,
				// Maybe user change the rtsp  port
				// Lock a.tasks than changed the css which in a.tasks, will be more safe
				a.lock.Lock()
				_ = a.refreshStreamUrl(css)
				atomic.StoreInt32(&css.RunningStatus, model.CloudStorageTaskStandby)
				a.lock.Unlock()
			}
			a.logger.Err(result.Err).Msgf("camera id:%d, task id:%s is done", css.CameraId, a.getTaskKey(css))
		}
	}
}

func (a *ArchiveTaskRunner) uploadSegments() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		<-ticker.C
		videos := a.db.FindArchiveVideosWith(db.NotUpload)
		for _, v := range videos {
			if _, err := os.Stat(v.FilePath); err != nil {
				a.db.DeleteArchiveVideos([]int64{v.Id})
				continue
			}

			a.limiter.Enter()
			if err := a.uploadSegment(&v); err != nil {
				continue
			}
			a.db.UpdateArchiveVideoStatus(v.Id, db.Uploaded)
		}
	}
}

func (a *ArchiveTaskRunner) uploadSegment(msg *db.ArchiveVideo) error {
	defer a.limiter.Leave()
	defer os.Remove(msg.FilePath)
	var s3File *utils.S3File
	var err error
	ext := filepath.Ext(msg.FilePath)
	if len(ext) < 2 { // like .ts must >= 2
		a.logger.Warn().Msgf("camera id:%d uploadSegment invalid file ext: %s", msg.CameraID, msg.FilePath)
		return fmt.Errorf("camera id:%d invalid file ext: %s", msg.CameraID, msg.FilePath)
	}
	ext = ext[1:]
	for i := 0; i < maxRetry; i++ {
		s3File, err = a.device.UploadS3ByTokenName(msg.CameraID, msg.FilePath, 0, 0, ext, TokenNameCloudStorage)
		if err == nil {
			break
		}
		a.logger.Warn().Msgf("camera id:%d try to upload file to s3: %d times, err: %v", msg.CameraID, i+1, err)
		time.Sleep(time.Second * time.Duration(i<<2))
	}
	if err != nil {
		a.logger.Warn().Msgf("camera id:%d failed to upload file to s3: %s, err: %v", msg.CameraID, msg.FilePath, err)
		return err
	}

	// generate utc time string to cloud.
	utcLoc, _ := time.LoadLocation("UTC")
	m := &cloud.Media{Videos: &[]cloud.MediaVideo{{
		File: cloud.File{
			Meta: cloud.Meta{
				FileSize:    s3File.FileSize,
				Size:        []int{s3File.Height, s3File.Width},
				ContentType: "video/" + s3File.Format,
			},
			Key:    s3File.Key,
			Bucket: s3File.Bucket,
		},
		StartedAt: time.Unix(msg.StartTime, 0).In(utcLoc).Format(utils.CloudTimeLayout),
		EndedAt:   time.Unix(msg.EndTime, 0).In(utcLoc).Format(utils.CloudTimeLayout),
	}}}
	cli := a.device.CloudClient()
	for i := 0; i < maxRetry; i++ {
		// upload s3Info to cloud
		err = cli.UploadCameraVideo(msg.CameraID, "", m, msg.TaskType)
		if err == nil {
			break
		}
		a.logger.Warn().Msgf("camera id:%d try to upload camera video to cloud: %d times, err: %v", msg.CameraID, i+1, err)
		time.Sleep(time.Second * time.Duration(i<<2))
	}
	return err
}

func (a *ArchiveTaskRunner) metric() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	recoverf := func() {
		end := time.Now().Add(-maxRecoverDurations)
		start := end.Add(-maxRecoverDurations)
		for _, t := range a.tasks {
			if t.Status == taskStatusOff {
				continue
			}
			// recover the segments only after task create time or task start time.
			if start.Unix() < t.CreatedTime && start.Unix() < t.StartTime {
				continue
			}
			videos := a.db.FindArchiveVideosIn(t.CameraId, start.Unix(), end.Unix())
			missings := a.findMissingRanges(videos, start.Unix(), end.Unix())
			var sum int64
			for _, m := range missings {
				if m.EndTime-m.StartTime < minRecoverySeconds {
					continue
				}
				sum += m.EndTime - m.StartTime
			}
			metrics.CounterVecAdd("cloud_storage_missing", []string{"camera_id"},
				[]string{strconv.Itoa(t.CameraId)}, float64(sum))
		}
	}
	for {
		recoverf()
		<-ticker.C
	}
}

func (a *ArchiveTaskRunner) HandleArchiveTasks() {
	for {
		// if disk is full
		diskUsage, err := utils.GetDiskUsage(DiskUsagePath)
		if nil == err {
			a.logger.Info().Msgf("disk usage:%d, task size:%d", diskUsage, len(a.tasks))
			if diskUsage >= atr.cloudStoragePauseDiskUsage {
				a.logger.Info().Msgf("disk usage:%d >= %d, pause cloud storage", diskUsage, atr.cloudStoragePauseDiskUsage)
				if !isDiskFullNotified {
					//	Alarm
					atime := time.Now().Format(utils.CloudTimeLayout)
					alarmErr := a.device.CloudClient().UploadAlarmInfo(&cloud.AlarmInfo{
						Detection: cloud.Detection{
							Algos: cloud.AlarmTypeBoxDiskFull,
						},
						Source:    cloud.AlarmSourceBridge,
						BoxId:     a.device.GetBoxId(),
						StartedAt: atime,
						EndedAt:   atime,
						Metadata:  cloud.AlarmMetaData{},
					})
					if alarmErr == nil {
						isDiskFullNotified = true
					}
				}
				isDiskFull = true
			} else if diskUsage <= atr.cloudStorageResumeDiskUsage {
				a.logger.Info().Msgf("disk usage:%d <= %d, resume cloud storage", diskUsage, atr.cloudStorageResumeDiskUsage)
				isDiskFull = false
				isDiskFullNotified = false
			}
		} else {
			a.logger.Error().Msgf("disk usage find err:%v", err)
		}

		a.lock.Lock()
		for _, task := range a.tasks {
			a.logger.Trace().Msgf("camera id:%d, task: %v", task.CameraId, task)
			if isDiskFull {
				if model.CloudStorageRunning == task.RunningStatus {
					// if disk is full, cancel the task
					task.Cancel()
					a.logger.Warn().Msgf("camera id:%d, task id:%s stop cloud storage", task.CameraId, a.getTaskKey(task))
				}
				task.RunningStatus = model.CloudStorageFull
				continue
			} else if model.CloudStorageFull == task.RunningStatus {
				// if disk is normal, reset the task status
				task.RunningStatus = model.CloudStorageTaskStandby
			}

			if task.TaskType == taskTypeRecovery {
				// compute recovery running task num
				var recoveryRunningTaskNum int
				for _, taskTmp := range a.tasks {
					if taskTmp.TaskType == taskTypeRecovery && taskTmp.RunningStatus == model.CloudStorageRunning {
						recoveryRunningTaskNum++
					}
				}

				// limit taskTypeRecovery number is case of rtsp 500 error
				if recoveryRunningTaskNum <= atr.cloudStorageTaskTypeRecoveryMaxNumber {
					a.processRecoveryTask(task)
				}
				continue
			}
			if task.IsLiveMode {
				a.processLiveTask(task)
			} else {
				a.processReplayTask(task)
			}
		}
		a.lock.Unlock()
		time.Sleep(archiveSchedulerHeartbeat)
		a.logger.Info().Msgf("current task:%+v, len:%d", a.tasks, len(a.tasks))
	}
}

func (a *ArchiveTaskRunner) TryRecover() {
	f := func() {
		defer func() {
			err := recover()
			if err != nil {
				a.logger.Error().Msgf("recover and sync panic: %v", err)
			}
		}()
		cli := a.device.CloudClient()
		if cli == nil {
			a.logger.Warn().Msgf("there's no cloud connection api")
			return
		}
		settings, err := cli.GetArchiveSetting()
		if err != nil {
			a.logger.Err(err).Msg("failed to get archive settings")
			return
		}

		for _, s := range settings {
			if err := a.UpdateArchiveSettings(s); err != nil {
				a.logger.Err(err).Msg("failed to update archive settings")
			}
		}
	}
	// recover and sync
	go func() {
		f()
		ticker := time.NewTicker(syncSettingsInterval)
		for {
			select {
			case <-ticker.C:
				f()
			}
		}
	}()
}

func (a *ArchiveTaskRunner) processReplayTask(task *model.CloudStorageSetting) {
	if task == nil {
		return
	}
	now := time.Now().Unix()
	startTime, endTime := a.getTaskStartEndTime(now, task.StartTime, task.EndTime)
	if startTime == 0 {
		return
	}
	// if start time is modified, then we may need stop task in advance.
	if task.RunningStatus == model.CloudStorageRunning &&
		now > endTime {
		//stop task.
		task.Cancel()
		task.RunningStatus = model.CloudStorageTaskStandby
		a.logger.Info().Msgf("camera id:%d task id:%s is stopped due to timeout", task.CameraId, a.getTaskKey(task))
		return
	}

	if task.RunningStatus == model.CloudStorageTaskStandby &&
		now > startTime && now < endTime {
		// start task
		task.StreamStartTime, task.StreamEndTime = a.getStreamStartEndTime(task.LastUpdateTime, startTime)
		task.Ctx, task.Cancel = context.WithTimeout(context.Background(), time.Duration(endTime-startTime)*time.Second)
		processor, err := streamer.NewArchiver(task.Ctx, task, a.outputChan, a.stopNotify, streamer.WithDataDir(a.device.GetConfig().GetDataStoreDir()))
		if err != nil {
			a.logger.Err(err).Msgf("camera id:%d failed to start archiver task id:%s", task.CameraId, a.getTaskKey(task))
			return
		}
		go processor.Start()
		task.RunningStatus = model.CloudStorageRunning
		a.logger.Info().Msgf("processReplayTask camera id:%d, task id:%s is started", task.CameraId, a.getTaskKey(task))
	}
}

// UpdateArchiveSettings need param check before calling UpdateArchiveSettings.
// if task is running, deleting task should not affect the running task as it is uploading yesterday's video.
func (a *ArchiveTaskRunner) UpdateArchiveSettings(task *model.CloudStorageSetting) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	taskKey := a.getTaskKey(task)
	prevTask, ok := a.tasks[taskKey]
	// this is new task.
	if !ok {
		// but if there's no existing task, task status is on, then we should create new one.
		if task.Status == taskStatusOn {
			a.createNewTask(task)
		}
		// but if it is task status off, it should be ignored.
		return nil
	}

	// prev task exist, it means the prev task is turned on.
	if task.Status == taskStatusOff {
		if prevTask.Cancel != nil {
			prevTask.Cancel()
		}
		delete(a.tasks, taskKey)
		a.logger.Info().Msgf("camera id:%d stop task and delete task id:%s", task.CameraId, a.getTaskKey(task))
		return nil
	}

	// if task is recording, we should update these values.
	if !task.IsLiveMode {
		if task.StartTime > 0 {
			atomic.StoreInt64(&prevTask.StartTime, task.StartTime)
		}
		if task.EndTime > 0 {
			atomic.StoreInt64(&prevTask.EndTime, task.EndTime)
		}
		if task.LastUpdateTime != 0 && task.LastUpdateTime < prevTask.LastUpdateTime && task.RunningStatus == model.CloudStorageTaskStandby {
			atomic.StoreInt64(&prevTask.LastUpdateTime, task.LastUpdateTime)
		}
	}

	// if resolution is changed, we should close current and renew one.
	// or if task live mode is changed, we should close current and renew one.
	if task.Resolution != "" && task.Resolution != prevTask.Resolution ||
		task.IsLiveMode != prevTask.IsLiveMode {
		if prevTask.Cancel != nil {
			prevTask.Cancel()
		}
		task.CreatedTime = prevTask.CreatedTime
		a.createNewTask(task)
		return nil
	}

	return nil
}

func (a *ArchiveTaskRunner) refreshStreamUrl(task *model.CloudStorageSetting) error {
	cam, err := a.device.GetCamera(task.CameraId)
	if err != nil {
		a.logger.Err(err).Msgf("failed to load camera id:%d, delete task id: %s", task.CameraId, a.getTaskKey(task))
		delete(a.tasks, a.getTaskKey(task))
		return err
	}
	task.StreamUrl, err = a.getStreamUrl(task, cam)
	if err != nil {
		a.logger.Err(err).Msgf("camera id:%d, task id:%s failed to get stream url", task.CameraId, a.getTaskKey(task))
		return err
	}
	return nil
}

func (a *ArchiveTaskRunner) createNewTask(task *model.CloudStorageSetting) error {
	err := a.refreshStreamUrl(task)
	if err != nil {
		return err
	}
	task.RunningStatus = model.CloudStorageTaskStandby
	if task.CreatedTime == 0 {
		task.CreatedTime = task.CreatedAt.Unix()
	}
	a.tasks[a.getTaskKey(task)] = task
	a.logger.Debug().Msgf("createNewTask :%v succeed", task)

	return nil
}

// getStreamStartEndTime returns the stream start and end time
// it will use lastUpdateTime only if lastUpdateTime is after stream start time.
func (a *ArchiveTaskRunner) getStreamStartEndTime(lastUpdateTime, startTime int64) (start, end int64) {
	start = startTime - day
	if lastUpdateTime > start {
		start = lastUpdateTime
	}
	end = startTime
	return
}

func (a *ArchiveTaskRunner) getTaskStartEndTime(now, startTime, endTime int64) (start, end int64) {
	if now < startTime {
		return
	}
	gap := now - startTime
	days := gap / day
	start = startTime + days*day
	end = start + endTime - startTime
	return start, end
}

func (a *ArchiveTaskRunner) getStreamUrl(setting *model.CloudStorageSetting, cam base.Camera) (u string, err error) {
	if cam.GetBrand() != utils.Uniview {
		return "", fmt.Errorf("getStreamUrl: unsupported camera branch:%s", cam.GetBrand())
	}
	var uri string
	switch utils.Resolution(setting.Resolution) {
	case utils.Normal:
		uri = cam.GetUri()
	case utils.HD:
		uri = cam.GetHdUri()
	case utils.SD:
		uri = cam.GetSdUri()
	default:
		uri = cam.GetUri()
	}
	if setting.IsLiveMode {
		u, err := url.Parse(uri)
		if err != nil {
			return "", fmt.Errorf("getStreamUrl:%v", err)
		}
		u.User = url.UserPassword(cam.GetUserName(), cam.GetPassword())
		return u.String(), nil
	} else {
		aiCam := cam.(base.AICamera)
		return aiCam.GetPlaybackUrlPattern(uri)
	}
}

func (a *ArchiveTaskRunner) getTaskKey(task *model.CloudStorageSetting) string {
	var ret = fmt.Sprintf("%d", task.Id)
	if task.TaskType == taskTypeRecovery {
		ret = fmt.Sprintf("%s-recovery-%d:%d", ret, task.StreamStartTime, task.StreamEndTime)
	}
	return ret
}
