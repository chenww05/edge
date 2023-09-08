package box

import (
	"context"

	"github.com/turingvideo/streamer"
	"github.com/turingvideo/turing-common/model"
)

func (a *ArchiveTaskRunner) processLiveTask(task *model.CloudStorageSetting) {
	if task == nil {
		return
	}
	if task.RunningStatus != model.CloudStorageTaskStandby {
		return
	}

	task.Ctx, task.Cancel = context.WithCancel(context.Background())
	processor, err := streamer.NewArchiver(task.Ctx, task, a.outputChan, a.stopNotify, streamer.WithDataDir(a.device.GetConfig().GetDataStoreDir()))
	if err != nil {
		a.logger.Err(err).Msgf("failed to start archiver task: %s", a.getTaskKey(task))
		return
	}
	go processor.Start()
	task.RunningStatus = model.CloudStorageRunning
	a.logger.Info().Msgf("task :%s, camera: %d is started", a.getTaskKey(task), task.CameraId)
}
