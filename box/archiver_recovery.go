package box

import (
	"context"
	"os"
	"sort"
	"time"

	"github.com/example/streamer"
	"github.com/example/turing-common/model"

	"github.com/example/minibox/db"
)

func (a *ArchiveTaskRunner) processRecoveryTask(task *model.CloudStorageSetting) {
	if task == nil {
		return
	}
	if task.RunningStatus != model.CloudStorageTaskStandby {
		return
	}
	timeout := task.StreamEndTime + int64(maxRecoverDurations.Seconds()) - time.Now().Unix()
	task.Ctx, task.Cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	processor, err := streamer.NewArchiver(task.Ctx, task, a.outputChan, a.stopNotify, streamer.WithDataDir(a.device.GetConfig().GetDataStoreDir()))
	if err != nil {
		a.logger.Err(err).Msgf("failed to start archiver task: %s", a.getTaskKey(task))
		return
	}
	go processor.Start()
	task.RunningStatus = model.CloudStorageRunning
	a.logger.Info().Msgf("task :%s, camera: %d is started, timeout after: %d", a.getTaskKey(task), task.CameraId, timeout)
}

func (a *ArchiveTaskRunner) recoverSegments() {
	// Delay 2 min for live task start
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	recoverf := func() {
		recoverEndTime := time.Now()
		minRecoverStartTime := recoverEndTime.Add(-maxRecoverDurations)
		for _, t := range a.tasks {
			if t.Status == taskStatusOff {
				continue
			}
			// If task is recovery just continue, we only need try to recover one time for one camera
			if t.TaskType == taskTypeRecovery {
				continue
			}
			// recover the segments only after task create time or task start time.
			recoverStartTimeTs := minRecoverStartTime.Unix()
			if minRecoverStartTime.Unix() < t.CreatedTime {
				recoverStartTimeTs = t.CreatedTime
			} else {
				recoverStartTimeTs = minRecoverStartTime.Unix()
			}
			videos := a.db.FindArchiveVideosIn(t.CameraId, recoverStartTimeTs, recoverEndTime.Unix())
			ranges := a.findMissingRanges(videos, recoverStartTimeTs, recoverEndTime.Unix())
			go a.recoverOneTask(ranges, t)
		}
	}
	isFirst := true
	for {
		<-ticker.C
		recoverf()
		if isFirst {
			ticker.Reset(time.Hour)
			isFirst = false
		}
	}
}

func (a *ArchiveTaskRunner) recoverOneTask(missingRanges []db.VideoStartEnd, task *model.CloudStorageSetting) {
	var recoveringTasks []model.CloudStorageSetting
	for _, r := range a.tasks {
		if r.Id == task.Id && r.TaskType == taskTypeRecovery {
			recoveringTasks = append(recoveringTasks, *r)
		}
	}
	sort.Slice(recoveringTasks, func(i, j int) bool {
		return recoveringTasks[i].StreamStartTime < recoveringTasks[j].StreamStartTime
	})
	sort.Slice(missingRanges, func(i, j int) bool {
		return missingRanges[i].StartTime < missingRanges[j].StartTime
	})

	// only need check the max end bound of existing recovery task.
	// we can assume the existing task is ongoing except they are completed.
	var temp []db.VideoStartEnd
	var maxRecoveryBound int64 = 0
	if len(recoveringTasks) > 0 {
		maxRecoveryBound = recoveringTasks[len(recoveringTasks)-1].StreamEndTime
	}

	for i := 0; i < len(missingRanges); i++ {
		if missingRanges[i].StartTime >= maxRecoveryBound {
			temp = append(temp, missingRanges[i:]...)
			break
		}
	}

	for _, r := range temp {
		if r.EndTime-r.StartTime < minRecoverySeconds {
			continue
		}
		now := time.Now()
		m := model.CloudStorageSetting{
			Id:              task.Id,
			CameraId:        task.CameraId,
			StartTime:       now.Unix(), // Ignore
			EndTime:         now.Unix(), // Ignore
			Status:          taskStatusOn,
			Resolution:      task.Resolution,
			IsLiveMode:      false,
			CreatedAt:       now,
			UpdatedAt:       now,
			TaskType:        taskTypeRecovery,
			StreamStartTime: r.StartTime,
			StreamEndTime:   r.EndTime,
		}
		a.createNewTask(&m)
		a.logger.Info().Msgf("trying to recovery missing ranges for %s", a.getTaskKey(&m))
	}
	return
}

func (a *ArchiveTaskRunner) cleanupRecordsFiles(videos []db.ArchiveVideo) []int64 {
	ids := []int64{}
	for _, v := range videos {
		a.logger.Debug().Msgf("try to delete file: %s", v.FilePath)
		_, err := os.Stat(v.FilePath)
		if err != nil && os.IsNotExist(err) {
			a.logger.Warn().Msgf("file not existed: %s", v.FilePath)
			ids = append(ids, v.Id)
			continue
		}
		err = os.Remove(v.FilePath)
		if err != nil {
			a.logger.Error().Msgf("delete file failed: %s", err.Error())
		} else {
			ids = append(ids, v.Id)
		}
	}
	return ids
}

func (a *ArchiveTaskRunner) cleanupRecords() {
	ticker := time.NewTicker(time.Hour)
	for {
		<-ticker.C
		go func() {
			end := time.Now().Add(-48 * time.Hour).Unix()
			a.logger.Info().Msgf("clean archive video before: %v", time.Unix(end, 0))
			videos := a.db.FindArchiveVideosBefore(end)
			ids := a.cleanupRecordsFiles(videos)
			_ = a.db.DeleteArchiveVideos(ids)
		}()
	}
}

func (a *ArchiveTaskRunner) findMissingRanges(videos []db.VideoStartEnd, start, end int64) []db.VideoStartEnd {
	if len(videos) == 0 {
		return []db.VideoStartEnd{{start, end}}
	}

	var ret []db.VideoStartEnd
	var last int64
	for _, v := range videos {
		if last != 0 && v.StartTime-last >= minRecoverySeconds {
			ret = append(ret, db.VideoStartEnd{StartTime: last, EndTime: v.StartTime})
		}
		if v.EndTime > last {
			last = v.EndTime
		}
	}

	if len(ret) > 0 {
		a.logger.Debug().Msgf("findMissingRanges output missing videos: %+v", ret)
	}

	return ret
}
