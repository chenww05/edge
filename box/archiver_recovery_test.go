package box

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/turingvideo/minibox/configs"
	"github.com/turingvideo/minibox/db"
	"github.com/turingvideo/minibox/mock"
)

func createTestTsData(t *testing.T, miss bool) []db.ArchiveVideo {
	s, err := os.Stat("/tmp/test_data")
	if err == nil {
		assert.True(t, s.IsDir())
	} else {
		assert.Nil(t, (os.Mkdir("/tmp/test_data", os.ModePerm)))
	}

	s, _ = os.Stat("/tmp/test_data")
	if s.IsDir() {
		file, _ := os.OpenFile("/tmp/test_data/1652171003.mp4", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		defer file.Close()
		file, _ = os.OpenFile("/tmp/test_data/1652171183.mp4", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		defer file.Close()
		file, _ = os.OpenFile("/tmp/test_data/1652171363.mp4", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		defer file.Close()
		file, _ = os.OpenFile("/tmp/test_data/1652171423.mp4", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		defer file.Close()
		if !miss {
			file, _ = os.OpenFile("/tmp/test_data/1652171483.mp4", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
			defer file.Close()
		}
	}

	videos := []db.ArchiveVideo{
		{
			Id: 1, CameraID: 123, TaskId: 123,
			TaskType: taskTypeRecovery, StartTime: 1652170944, EndTime: 1652171003,
			FilePath: "/tmp/test_data/1652171003.mp4", Uploaded: 1, CreatTime: 1652171003,
		},
		{
			Id: 2, CameraID: 123, TaskId: 123,
			TaskType: taskTypeRecovery, StartTime: 1652171123, EndTime: 1652171183,
			FilePath: "/tmp/test_data/1652171183.mp4", Uploaded: 1, CreatTime: 1652171183,
		},
		{
			Id: 3, CameraID: 123, TaskId: 123,
			TaskType: taskTypeRecovery, StartTime: 1652171303, EndTime: 1652171363,
			FilePath: "/tmp/test_data/1652171363.mp4", Uploaded: 1, CreatTime: 1652171363,
		},
		{
			Id: 4, CameraID: 123, TaskId: 123,
			TaskType: taskTypeRecovery, StartTime: 1652171363, EndTime: 1652171423,
			FilePath: "/tmp/test_data/1652171423.mp4", Uploaded: 1, CreatTime: 1652171423,
		},
		{
			Id: 5, CameraID: 123, TaskId: 123,
			TaskType: taskTypeRecovery, StartTime: 1652171423, EndTime: 1652171483,
			FilePath: "/tmp/test_data/1652171483.mp4", Uploaded: 1, CreatTime: 1652171483,
		},
	}
	return videos
}

func Test_ArchiveRecoveryTask(t *testing.T) {
	var config configs.CloudStorageConfig
	t.Run("delete files", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		b := mock.NewMockBox(ctrl)
		mdb := mock.NewMockDBClient(ctrl)
		atr := NewArchiveTaskRunner(b, mdb, config)

		videos := createTestTsData(t, false)
		ids := atr.cleanupRecordsFiles(videos)
		assert.Equal(t, []int64{1, 2, 3, 4, 5}, ids)
		_ = os.RemoveAll("/tmp/test_data")
	})

	t.Run("delete files with missing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		b := mock.NewMockBox(ctrl)
		mdb := mock.NewMockDBClient(ctrl)
		atr := NewArchiveTaskRunner(b, mdb, config)

		videos := createTestTsData(t, true)
		ids := atr.cleanupRecordsFiles(videos)
		assert.Equal(t, []int64{1, 2, 3, 4, 5}, ids)
	})
}
