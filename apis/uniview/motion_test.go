package uniview

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/turingvideo/minibox/db"
	"github.com/turingvideo/turing-common/model"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	box2 "github.com/turingvideo/minibox/box"
	"github.com/turingvideo/minibox/camera/uniview"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/configs"
	"github.com/turingvideo/minibox/mock"
	"github.com/turingvideo/minibox/utils"
)

func TestMotionProcessIsMotionStart(t *testing.T) {
	t.Parallel()
	mp := newMotionProcess()
	t.Run("is true", func(t *testing.T) {
		res := mp.isMotionStart(1)
		assert.True(t, res)
	})
	t.Run("is false", func(t *testing.T) {
		mp.pushMotion(&motionEvent{cameraID: 1})
		res := mp.isMotionStart(1)
		assert.False(t, res)
	})
	t.Run("is true", func(t *testing.T) {
		mp.popAllMotions(1)
		res := mp.isMotionStart(1)
		assert.True(t, res)
	})
}

func TestMotionProcessPushMotion(t *testing.T) {
	mp := newMotionProcess()
	t.Run("camera id=1", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, mp.cameraMotionGroup[1])
		mp.pushMotion(&motionEvent{cameraID: 1})
		assert.Equal(t, len(mp.cameraMotionGroup[1].events), 1)
		mp.pushMotion(&motionEvent{cameraID: 1})
		assert.Equal(t, len(mp.cameraMotionGroup[1].events), 2)
	})
	t.Run("camera id=2", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, mp.cameraMotionGroup[2])
		mp.pushMotion(&motionEvent{cameraID: 2})
		assert.Equal(t, len(mp.cameraMotionGroup[2].events), 1)
		mp.pushMotion(&motionEvent{cameraID: 2})
		assert.Equal(t, len(mp.cameraMotionGroup[2].events), 2)
	})
}

func TestMotionProcessPopAllMotions(t *testing.T) {
	mp := newMotionProcess()
	t.Run("camera id=1", func(t *testing.T) {
		t.Parallel()
		events := mp.popAllMotions(1)
		assert.Equal(t, len(events), 0)
		assert.Nil(t, mp.cameraMotionGroup[1])
		mp.pushMotion(&motionEvent{cameraID: 1})
		assert.Equal(t, len(mp.cameraMotionGroup[1].events), 1)
		mp.pushMotion(&motionEvent{cameraID: 1})
		assert.Equal(t, len(mp.cameraMotionGroup[1].events), 2)
		events = mp.popAllMotions(1)
		assert.Nil(t, mp.cameraMotionGroup[1])
		assert.Equal(t, len(events), 2)
	})
	t.Run("camera id=2", func(t *testing.T) {
		t.Parallel()
		events := mp.popAllMotions(2)
		assert.Equal(t, len(events), 0)
		assert.Nil(t, mp.cameraMotionGroup[2])
		mp.pushMotion(&motionEvent{cameraID: 2})
		assert.Equal(t, len(mp.cameraMotionGroup[2].events), 1)
		mp.pushMotion(&motionEvent{cameraID: 2})
		assert.Equal(t, len(mp.cameraMotionGroup[2].events), 2)
		events = mp.popAllMotions(2)
		assert.Nil(t, mp.cameraMotionGroup[2])
		assert.Equal(t, len(events), 2)
	})
}

func TestMotionProcessUploadMotionEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	cfg := configs.NewEmptyConfig()
	box.EXPECT().GetConfig().Return(&cfg).AnyTimes()
	box.EXPECT().UploadS3ByTokenName(10, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).AnyTimes()
	box.EXPECT().UploadAICameraEvent(10, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("test_event_id", nil).AnyTimes()
	box.EXPECT().UploadEventMedia("test_event_id", gomock.Any()).Return(nil).AnyTimes()
	box.EXPECT().NotifyCloudEventVideoClipUploadFailed(gomock.Any()).Return(nil).AnyTimes()

	box.EXPECT().GetConfig().Return(&cfg).AnyTimes()
	imageList := []ImageInfo{{Data: data}}

	dbCli := mock.NewMockDBClient(ctrl)
	dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).AnyTimes()
	dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).AnyTimes()
	dbCli.EXPECT().UpdateEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	dbCli.EXPECT().UpdateEventVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	u := getMockUniviewApi(box, dbCli)
	t.Run("success upload", func(t *testing.T) {
		t.Parallel()
		c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
		univCam := c.(*uniview.BaseUniviewCamera)
		univCam.DetectParams = model.DetectParams{
			DetectAt:            "cloud",
			CarThreshold:        0.2,
			PersonThreshold:     0.2,
			MotorcycleThreshold: 0.2,
		}
		cloud.SaveCameraSettings([]*cloud.CameraSettings{
			{
				CamID:           c.GetID(),
				CloudEventTypes: []string{cloud.MotionStart},
			},
		})
		mp := newMotionProcess()
		err := mp.uploadMotionStartEvent(&motionEvent{cameraID: 1}, u, univCam, imageList)
		assert.Nil(t, err)
	})
	t.Run("upload error", func(t *testing.T) {
		t.Parallel()
		c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
		cloud.SaveCameraSettings([]*cloud.CameraSettings{
			{
				CamID:           c.GetID(),
				CloudEventTypes: []string{cloud.MotionStart},
			},
		})
		univCam := c.(*uniview.BaseUniviewCamera)
		univCam.DetectParams = model.DetectParams{
			DetectAt:            "null",
			CarThreshold:        0.2,
			PersonThreshold:     0.2,
			MotorcycleThreshold: 0.2,
		}
		cloud.SaveCameraSettings([]*cloud.CameraSettings{
			{
				CamID:           c.GetID(),
				CloudEventTypes: []string{cloud.MotionStart},
			},
		})
		t.Run("run error", func(t *testing.T) {
			mp := newMotionProcess()
			err := mp.uploadMotionStartEvent(&motionEvent{cameraID: 1}, u, univCam, nil)
			assert.NotNil(t, err)
		})
		t.Run("ErrNoRemoteID", func(t *testing.T) {
			mp := newMotionProcess()
			err := mp.uploadMotionStartEvent(&motionEvent{cameraID: 1}, u, univCam, imageList)
			assert.Equal(t, ErrNoRemoteID, err)
		})
	})
}

func TestMotionProcess_genProtocolFile(t *testing.T) {
	mp := newMotionProcess()
	t.Run("no images", func(t *testing.T) {
		events := motionEvents{}
		events = append(events, &motionEvent{})
		filename, imgPaths, err := mp.genProtocolFile(events, "./")
		t.Log(filename, imgPaths, err)
		defer func() {
			if len(filename) > 0 {
				_ = os.Remove(filename)
			}
			for _, imgPath := range imgPaths {
				if len(imgPath) > 0 {
					_ = os.Remove(imgPath)
				}
			}
		}()
		assert.Equal(t, err, ErrNoImages)
	})

	t.Run("success", func(t *testing.T) {
		events := motionEvents{}
		events = append(events, &motionEvent{imgBase64: data}, &motionEvent{imgBase64: data})
		filename, imgPaths, err := mp.genProtocolFile(events, "./")
		t.Log(filename, imgPaths, err)
		defer func() {
			if len(filename) > 0 {
				_ = os.Remove(filename)
			}
			for _, imgPath := range imgPaths {
				if len(imgPath) > 0 {
					_ = os.Remove(imgPath)
				}
			}
		}()
		var content []byte
		for _, v := range imgPaths {
			content = append(content, []byte(fmt.Sprintf("file './%s'\nduration %.2f\n", path.Base(v), 1.00))...)
		}
		lastImg := imgPaths[len(imgPaths)-1]
		content = append(content, []byte(fmt.Sprintf("file './%s'\n", path.Base(lastImg)))...)
		buff, _ := ioutil.ReadFile(filename)
		t.Log(string(buff))
		t.Log(string(content))
		assert.EqualValues(t, string(content), string(buff))
	})
}

func TestMotionProcess_compressSnapshots(t *testing.T) {
	mp := newMotionProcess()
	t.Run("no images", func(t *testing.T) {
		events := motionEvents{}
		events = append(events, &motionEvent{})
		filename, err := mp.compressSnapshots(events, "./")
		t.Log(filename, err)
		defer func() {
			if len(filename) > 0 {
				_ = os.Remove(filename)
			}
		}()
		assert.Equal(t, err, ErrNoImages)
	})

	t.Run("compress error", func(t *testing.T) {
		events := motionEvents{}
		events = append(events, &motionEvent{imgBase64: data}, &motionEvent{imgBase64: data})
		filename, err := mp.compressSnapshots(events, "./")
		t.Log(filename, err)
		defer func() {
			if len(filename) > 0 {
				_ = os.Remove(filename)
			}
		}()

	})
}

func TestMotionProcess_mergeEvents(t *testing.T) {
	mp := newMotionProcess()
	t.Run("no events", func(t *testing.T) {
		events := motionEvents{}
		merged, videoPath, err := mp.mergeEvents(events, "./")
		t.Log(merged, videoPath, err)
		assert.Equal(t, ErrNoMotionEvent, err)
	})
	t.Run("no images", func(t *testing.T) {
		events := motionEvents{}
		events = append(events, &motionEvent{})
		merged, videoPath, err := mp.mergeEvents(events, "./")
		t.Log(merged, videoPath, err)
		assert.Equal(t, ErrNoImages, err)
	})
	t.Run("ffmpeg error", func(t *testing.T) {
		events := motionEvents{}
		events = append(events, &motionEvent{imgBase64: data})
		merged, videoPath, err := mp.mergeEvents(events, "./")
		defer func() {
			if len(videoPath) > 0 {
				_ = os.Remove(videoPath)
			}
		}()
		t.Log(merged, videoPath, err)
		assert.NotNil(t, err)
	})
}

func TestMotionProcess_uploadMotionEventsVideo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	cfg := mock.NewMockConfig(ctrl)
	cfg.EXPECT().GetDataStoreDir().Return("./").AnyTimes()
	dbCli := mock.NewMockDBClient(ctrl)
	box.EXPECT().GetConfig().Return(cfg).AnyTimes()
	u := getMockUniviewApi(box, dbCli)

	mp := newMotionProcess()
	t.Run("no images", func(t *testing.T) {
		c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, cfg)
		univCam := c.(*uniview.BaseUniviewCamera)
		univCam.DetectParams = model.DetectParams{
			DetectAt:            "cloud",
			CarThreshold:        0.2,
			PersonThreshold:     0.2,
			MotorcycleThreshold: 0.2,
		}
		cloud.SaveCameraSettings([]*cloud.CameraSettings{
			{
				CamID:           c.GetID(),
				CloudEventTypes: []string{cloud.MotionStart},
			},
		})
		events := motionEvents{}
		events = append(events, &motionEvent{})
		err := mp.uploadMotionEventsVideo(events, u, univCam)
		t.Log(err)
		assert.Equal(t, ErrNoImages, err)
	})
	t.Run("ffmpeg error", func(t *testing.T) {
		c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, cfg)
		univCam := c.(*uniview.BaseUniviewCamera)
		univCam.DetectParams = model.DetectParams{
			DetectAt:            "cloud",
			CarThreshold:        0.2,
			PersonThreshold:     0.2,
			MotorcycleThreshold: 0.2,
		}
		events := motionEvents{}
		events = append(events, &motionEvent{imgBase64: data})
		events = append(events, &motionEvent{imgBase64: data})
		err := mp.uploadMotionEventsVideo(events, u, univCam)
		t.Log(err)
		assert.NotNil(t, err)
	})
}
