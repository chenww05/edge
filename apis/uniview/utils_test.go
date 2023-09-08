package uniview

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/example/turing-common/model"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	univiewapi "github.com/example/goshawk/uniview"

	"github.com/example/minibox/apis/structs"
	box2 "github.com/example/minibox/box"
	"github.com/example/minibox/camera/base"
	"github.com/example/minibox/camera/uniview"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/configs"
	"github.com/example/minibox/db"
	"github.com/example/minibox/mock"
	"github.com/example/minibox/utils"
)

const data = `
/9j/4AAQSkZJRgABAQIAHAAcAAD/2wBDABALDA4MChAODQ4SERATGCgaGBYWGDEjJR0oOjM9PDkzODdA
SFxOQERXRTc4UG1RV19iZ2hnPk1xeXBkeFxlZ2P/2wBDARESEhgVGC8aGi9jQjhCY2NjY2NjY2NjY2Nj
Y2NjY2NjY2NjY2NjY2NjY2NjY2NjY2NjY2NjY2NjY2NjY2NjY2P/wAARCABnAJYDASIAAhEBAxEB/8QA
HwAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoL/8QAtRAAAgEDAwIEAwUFBAQAAAF9AQIDAAQRBRIh
MUEGE1FhByJxFDKBkaEII0KxwRVS0fAkM2JyggkKFhcYGRolJicoKSo0NTY3ODk6Q0RFRkdISUpTVFVW
V1hZWmNkZWZnaGlqc3R1dnd4eXqDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXG
x8jJytLT1NXW19jZ2uHi4+Tl5ufo6erx8vP09fb3+Pn6/8QAHwEAAwEBAQEBAQEBAQAAAAAAAAECAwQF
BgcICQoL/8QAtREAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAV
YnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOE
hYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq
8vP09fb3+Pn6/9oADAMBAAIRAxEAPwDlwKMD0pwzSiuK57QzGDxS7D6in8Y5ximnAPUfSlcq4m3ilUYp
2OKXHvRcVxnTtS7c07HNFK4DQPakC4PNOA+tOx70XAjK/So5gBGP94fzqfvUVx/qxx/EP51UXqRP4WSE
cmgjilP3jSEZqS0IO/NGDnpUiocDg/McDjvV6HTPOdVWYgsM5KcfzzQ2JySM2jp6VYu7SWzmMUwG4cgj
kMPUVBjjtTGtRu0Zopw+lFFxhinrGzuqqMsxAA9yaXFSRv5cqSEcIwYj6GpuZ30O30fSLKzhUpbpNMv3
5XGTn29BV28jt7pPLuIVljPBBFVreYx+VbqAjycgt3x14zRcNOxGyVFHQkIc/wA61exyKLbuzjdZ046d
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
`

var mockError = errors.New("mock error")

func Test_parseNVRSN(t *testing.T) {
	type args struct {
		notification EventNotification
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "success",
			args: args{
				notification: EventNotification{Reference: "ttt/122123"},
			},
			want: "122123",
		},
		{
			name: "null sn",
			args: args{
				notification: EventNotification{Reference: "ttt/"},
			},
			want: "",
		},
		{
			name: "null sn not have /",
			args: args{
				notification: EventNotification{Reference: "ttt"},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseNVRSN(tt.args.notification.Reference); got != tt.want {
				t.Errorf("parseNVRSN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUniviewAPI_getCamera(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("found camera success", func(t *testing.T) {
		t.Parallel()

		box := mock.NewMockBox(ctrl)
		db := mock.NewMockDBClient(ctrl)
		cg := mock.NewMockCamGroup(ctrl)
		cfg := configs.NewEmptyConfig()

		c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
		c.SetNvrSN("test_sn_1")
		c.SetStatus(true)

		cg.EXPECT().AllCameras().Return([]base.Camera{c}).AnyTimes()
		box.EXPECT().GetCamGroup().Return(cg).AnyTimes()
		box.EXPECT().GetConfig().Return(&cfg).AnyTimes()
		u := getMockUniviewApi(box, db)

		univCam, err := u.getCamera("test_sn_1", 1)
		assert.Nil(t, err)
		assert.Equal(t, univCam, c)
	})
	t.Run("no camera found", func(t *testing.T) {
		t.Parallel()

		box := mock.NewMockBox(ctrl)
		db := mock.NewMockDBClient(ctrl)
		cg := mock.NewMockCamGroup(ctrl)
		cfg := configs.NewEmptyConfig()

		c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
		c.SetNvrSN("test_sn_1")
		c.SetStatus(true)

		cg.EXPECT().AllCameras().Return([]base.Camera{c}).AnyTimes()
		box.EXPECT().GetCamGroup().Return(cg).AnyTimes()
		box.EXPECT().GetConfig().Return(&cfg).AnyTimes()
		u := getMockUniviewApi(box, db)

		univCam, err := u.getCamera("test_sn_2", 1)
		assert.NotNil(t, err)
		assert.Nil(t, univCam)
	})
	t.Run("duplicated camera on the same channel", func(t *testing.T) {
		t.Parallel()

		box := mock.NewMockBox(ctrl)
		db := mock.NewMockDBClient(ctrl)
		cg := mock.NewMockCamGroup(ctrl)
		cfg := configs.NewEmptyConfig()
		u := getMockUniviewApi(box, db)

		box.EXPECT().GetCamGroup().Return(cg).AnyTimes()
		box.EXPECT().GetConfig().Return(&cfg).AnyTimes()

		online_camera := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
		online_camera.SetNvrSN("test_sn_1")
		online_camera.SetStatus(true)

		offline_camera := uniview.NewCamera(&cloud.Camera{ID: 11, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
		offline_camera.SetNvrSN("test_sn_1")
		offline_camera.SetStatus(false)

		cg.EXPECT().AllCameras().Return([]base.Camera{offline_camera, online_camera}).Times(1)

		univCam1, err := u.getCamera("test_sn_1", 1)
		assert.Nil(t, err)
		assert.Equal(t, univCam1, online_camera)

		cg.EXPECT().AllCameras().Return([]base.Camera{online_camera, offline_camera}).Times(1)
		univCam2, err := u.getCamera("test_sn_1", 1)
		assert.Nil(t, err)
		assert.Equal(t, univCam2, online_camera)

	})
}

func TestUniviewAPI_processEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	cfg := configs.NewEmptyConfig()
	c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
	univCam := c.(*uniview.BaseUniviewCamera)

	cloud.SaveCameraSettings([]*cloud.CameraSettings{
		{
			CamID:           c.GetID(),
			CloudEventTypes: []string{cloud.Car},
		},
	})
	box.EXPECT().GetConfig().Return(&cfg).AnyTimes()
	box.EXPECT().NotifyCloudEventVideoClipUploadFailed(gomock.Any()).Return(nil).AnyTimes()
	t.Run("saveEvent false,uploadCloud false", func(t *testing.T) {
		dbCli := mock.NewMockDBClient(ctrl)
		u := getMockUniviewApi(box, dbCli)
		u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, false, false, true, time.Now(), 7)
	})

	t.Run("saveEvent true", func(t *testing.T) {
		t.Run("saveEvent error", func(t *testing.T) {
			dbCli := mock.NewMockDBClient(ctrl)
			dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, mockError)
			u := getMockUniviewApi(box, dbCli)
			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, false, true, time.Now(), 7)
		})

		t.Run("uploadCloud false,other success", func(t *testing.T) {
			dbCli := mock.NewMockDBClient(ctrl)
			dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).AnyTimes()
			dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).AnyTimes()
			dbCli.EXPECT().UpdateEventVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			u := getMockUniviewApi(box, dbCli)
			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, false, true, time.Now(), 7)
		})
	})

	t.Run("uploadCloud true", func(t *testing.T) {
		dbCli := mock.NewMockDBClient(ctrl)
		dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).AnyTimes()
		dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).AnyTimes()
		dbCli.EXPECT().UpdateEventVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		box.EXPECT().UploadS3ByTokenName(10, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).AnyTimes()
		box.EXPECT().UploadS3ByTokenName(11, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, mockError).AnyTimes()
		box.EXPECT().UploadAICameraEvent(10, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("123", nil).AnyTimes()

		t.Run("UploadAICameraEvent error", func(t *testing.T) {
			u := getMockUniviewApi(box, dbCli)
			c2 := uniview.NewCamera(&cloud.Camera{ID: 11, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
			univCam2 := c2.(*uniview.BaseUniviewCamera)
			u.processEvent(univCam2, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, false, time.Now(), 7)
		})

		t.Run("UpdateEvent error", func(t *testing.T) {
			dbCli.EXPECT().UpdateEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockError).Times(1)
			u := getMockUniviewApi(box, dbCli)
			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, false, time.Now(), 7)
		})

		t.Run("uploadVideo false,UpdateEvent success", func(t *testing.T) {
			dbCli.EXPECT().UpdateEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			u := getMockUniviewApi(box, dbCli)
			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, false, time.Now(), 7)
		})
	})

	t.Run("uploadVideo ture", func(t *testing.T) {
		dbCli := mock.NewMockDBClient(ctrl)
		dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).AnyTimes()
		dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).AnyTimes()
		box.EXPECT().UploadS3ByTokenName(10, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).AnyTimes()
		box.EXPECT().UploadS3ByTokenName(11, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, mockError).AnyTimes()
		box.EXPECT().UploadAICameraEvent(10, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("123", nil).AnyTimes()
		box.EXPECT().UploadEventMedia(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		dbCli.EXPECT().UpdateEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		t.Run("NvrWriteCacheToDisk success,handleEventVideo error", func(t *testing.T) {
			box.EXPECT().GetCamera(10).Return(univCam, mockError).AnyTimes()
			nc := univiewapi.NewMockClient(ctrl)
			nc.EXPECT().GetChannelStreamsRecords(uint32(1), 2, gomock.Any(), gomock.Any()).Return(&univiewapi.RecordList{
				Nums:        1,
				RecordInfos: []univiewapi.RecordInfo{{Begin: 10}},
			}, nil).AnyTimes()
			_ = univCam.SetNVRClient(nc)
			t.Run("UpdateEventVideo error", func(t *testing.T) {
				aiCam := mock.NewMockAICamera(ctrl)
				aiCam.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("path.jpg", 1920, 1080, nil).AnyTimes()
				dbCli.EXPECT().UpdateEventVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockError).Times(1)
				box.EXPECT().GetCamera(gomock.Any()).Return(aiCam, nil).AnyTimes()
				u := getMockUniviewApi(box, dbCli)
				u.processEvent(univCam, EventCar, 100, data, &structs.MetaScanData{}, true, true, true, time.Now(), 7)
			})

			t.Run("UpdateEventVideo success", func(t *testing.T) {
				dbCli.EXPECT().UpdateEventVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				u := getMockUniviewApi(box, dbCli)
				u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, true, time.Now(), 7)
			})
		})
	})
}

func TestUniviewAPI_processEvent1(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	var b *mock.MockBox
	var cfg configs.BaseConfig
	var univCam *uniview.BaseUniviewCamera
	var dbCli *mock.MockDBClient
	var u *UniviewAPI
	init := func() {
		b = mock.NewMockBox(ctrl)
		cfg = configs.NewEmptyConfig()
		c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
		univCam = c.(*uniview.BaseUniviewCamera)
		dbCli = mock.NewMockDBClient(ctrl)
		u = getMockUniviewApi(b, dbCli)

		cloud.SaveCameraSettings([]*cloud.CameraSettings{
			{
				CamID:           c.GetID(),
				CloudEventTypes: []string{cloud.Car},
			},
		})
		b.EXPECT().GetConfig().Return(&cfg).AnyTimes()
		b.EXPECT().NotifyCloudEventVideoClipUploadFailed(gomock.Any()).Return(nil).AnyTimes()
	}

	t.Run("no license", func(t *testing.T) {
		init()
		u.processEvent(univCam, EventPeople, time.Now().Unix(), data, &structs.MetaScanData{}, false, false, true, time.Now(), 7)
	})

	t.Run("saveEvent,uploadCloud,uploadVideo", func(t *testing.T) {
		// "FFF" => saveEvent: False, uploadCloud: False, uploadVideo: False
		t.Run("FFF saveEvent: False, uploadCloud: False, uploadVideo: False", func(t *testing.T) {
			init()
			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, false, false, false, time.Now(), 7)
		})
		// "FFT" => saveEvent: False, uploadCloud: False, uploadVideo: True
		t.Run("FFT saveEvent: False, uploadCloud: False, uploadVideo: True", func(t *testing.T) {
			init()
			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, false, false, true, time.Now(), 7)
		})
		// "FTF" => saveEvent: False, uploadCloud: True, uploadVideo: False
		t.Run("FTF saveEvent: False, uploadCloud: True, uploadVideo: False", func(t *testing.T) {
			init()
			b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(nil, errors.New("error s3")).Times(1)
			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, false, true, false, time.Now(), 7)
		})
		// "TFF" => saveEvent: True, uploadCloud: False, uploadVideo: False
		t.Run("TFF saveEvent: True, uploadCloud: False, uploadVideo: False", func(t *testing.T) {
			t.Run("saveEventDB error", func(t *testing.T) {
				t.Run("save picture to db err", func(t *testing.T) {
					init()
					dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, errors.New("save picture to db err")).Times(1)
					u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, false, false, time.Now(), 7)
				})
				t.Run("create Event err", func(t *testing.T) {
					init()
					dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).Times(1)
					dbCli.EXPECT().CreateEvent(gomock.Any()).Return(errors.New("create Event error")).Times(1)
					u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, false, false, time.Now(), 7)
				})
			})
			t.Run("saveEventDB success", func(t *testing.T) {
				init()
				dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).Times(1)
				dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).Times(1)
				dbCli.EXPECT().UpdateEventVideo(uint(0), "", "", false).Return(nil).Times(1)
				u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, false, false, time.Now(), 7)
			})
		})
		// "FTT" => saveEvent: False, uploadCloud: True, uploadVideo: True
		t.Run("FTT saveEvent: False, uploadCloud: True, uploadVideo: True", func(t *testing.T) {
			t.Run("uploadEventToCloud error", func(t *testing.T) {
				t.Run("s3 error", func(t *testing.T) {
					init()
					b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(nil, errors.New("s3 error")).Times(1)
					u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, false, true, true, time.Now(), 7)
				})
				t.Run("cloud api err", func(t *testing.T) {
					init()
					b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
					b.EXPECT().UploadAICameraEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("cloud api error")).Times(1)
					u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, false, true, true, time.Now(), 7)
				})
			})
			t.Run("uploadEventToCloud success", func(t *testing.T) {
				t.Run("event video failed", func(t *testing.T) {
					init()
					b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
					b.EXPECT().UploadAICameraEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("uuid", nil).Times(1)
					b.EXPECT().GetCamera(gomock.Any()).Return(nil, errors.New("error camera"))
					u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, false, true, true, time.Now(), 7)
				})
				t.Run("event video ok", func(t *testing.T) {
					// upload event to cloud
					init()
					b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
					b.EXPECT().UploadAICameraEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("uuid", nil).Times(1)

					// handler event video
					aiCam := mock.NewMockAICamera(ctrl)
					b.EXPECT().GetCamera(gomock.Any()).Return(aiCam, nil).AnyTimes()
					aiCam.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
					aiCam.EXPECT().GetChannel().Return(uint32(1)).AnyTimes()
					aiCam.EXPECT().NvrWriteCacheToDisk(uint32(1), 2, gomock.Any(), gomock.Any()).Return(nil).Times(1)

					// record
					aiCam.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("path.jpg", 1920, 1080, nil).AnyTimes()

					// video to s3
					aiCam.EXPECT().GetID().Return(10).AnyTimes()
					b.EXPECT().UploadS3ByTokenName(10, "path.jpg", 1080, 1920, "mp4", box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
					b.EXPECT().UploadEventMedia("uuid", gomock.Any()).Return(nil).Times(1)

					u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, false, true, true, time.Now(), 7)
				})
			})

		})
		// "TFT" => saveEvent: True, uploadCloud: False, uploadVideo: True
		t.Run("TFT saveEvent: True, uploadCloud: False, uploadVideo: True", func(t *testing.T) {
			init()
			dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).Times(1)
			dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).Times(1)
			dbCli.EXPECT().UpdateEventVideo(uint(0), "", "", true).Return(nil).Times(1)
			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, false, true, time.Now(), 7)
		})
		// "TTF" => saveEvent: True, uploadCloud: True, uploadVideo: False
		t.Run("TTF saveEvent: True, uploadCloud: True, uploadVideo: False", func(t *testing.T) {
			init()
			dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).Times(1)
			dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).Times(1)
			b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
			b.EXPECT().UploadAICameraEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("uuid", nil).Times(1)
			dbCli.EXPECT().UpdateEvent(uint(0), "uuid", gomock.Any()).Return(nil).Times(1)
			dbCli.EXPECT().UpdateEventVideo(uint(0), "", "", false).Return(nil).Times(1)
			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, false, time.Now(), 7)
		})
		// "TTT" => saveEvent: True, uploadCloud: True, uploadVideo: True
		t.Run("TTT saveEvent: True, uploadCloud: True, uploadVideo: True", func(t *testing.T) {
			t.Run("save picture to db err", func(t *testing.T) {
				init()
				dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, errors.New("save picture to db err")).Times(1)

				// upload event to cloud
				b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
				b.EXPECT().UploadAICameraEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("uuid", nil).Times(1)

				// handler event video
				aiCam := mock.NewMockAICamera(ctrl)
				b.EXPECT().GetCamera(gomock.Any()).Return(aiCam, nil).AnyTimes()
				aiCam.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
				aiCam.EXPECT().GetChannel().Return(uint32(1)).AnyTimes()
				aiCam.EXPECT().NvrWriteCacheToDisk(uint32(1), 2, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

				// record
				aiCam.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("path.jpg", 1920, 1080, nil).AnyTimes()

				// video to s3
				aiCam.EXPECT().GetID().Return(10).AnyTimes()
				b.EXPECT().UploadS3ByTokenName(10, "path.jpg", 1080, 1920, "mp4", box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
				b.EXPECT().UploadEventMedia("uuid", gomock.Any()).Return(nil).Times(1)

				u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, true, time.Now(), 7)
			})

			t.Run("create Event err", func(t *testing.T) {
				init()
				dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).Times(1)
				dbCli.EXPECT().CreateEvent(gomock.Any()).Return(errors.New("create event err")).Times(1)

				// upload event to cloud
				b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
				b.EXPECT().UploadAICameraEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("uuid", nil).Times(1)

				// handler event video
				aiCam := mock.NewMockAICamera(ctrl)
				b.EXPECT().GetCamera(gomock.Any()).Return(aiCam, nil).AnyTimes()
				aiCam.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
				aiCam.EXPECT().GetChannel().Return(uint32(1)).AnyTimes()
				aiCam.EXPECT().NvrWriteCacheToDisk(uint32(1), 2, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

				// record
				aiCam.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("path.jpg", 1920, 1080, nil).AnyTimes()

				// video to s3
				aiCam.EXPECT().GetID().Return(10).AnyTimes()
				b.EXPECT().UploadS3ByTokenName(10, "path.jpg", 1080, 1920, "mp4", box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
				b.EXPECT().UploadEventMedia("uuid", gomock.Any()).Return(nil).Times(1)

				u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, true, time.Now(), 7)
			})
		})

		t.Run("snap to s3 err", func(t *testing.T) {
			init()
			dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).Times(1)
			dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).Times(1)

			// upload event to cloud
			b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, errors.New("err snap to s3")).Times(1)

			dbCli.EXPECT().UpdateEventVideo(uint(0), "", "", true).Return(nil).Times(1)
			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, true, time.Now(), 7)
		})

		t.Run("cloud event api err", func(t *testing.T) {
			init()
			dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).Times(1)
			dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).Times(1)

			// upload event to cloud
			b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
			b.EXPECT().UploadAICameraEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil).Times(1)

			dbCli.EXPECT().UpdateEventVideo(uint(0), "", "", true).Return(nil).Times(1)
			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, true, time.Now(), 7)
		})

		// err in handler event Video
		t.Run("record video err", func(t *testing.T) {
			init()
			dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).Times(1)
			dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).Times(1)

			// upload event to cloud
			b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
			b.EXPECT().UploadAICameraEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("uuid", nil).Times(1)

			// handler event video
			aiCam := mock.NewMockAICamera(ctrl)
			b.EXPECT().GetCamera(gomock.Any()).Return(aiCam, nil).AnyTimes()
			aiCam.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
			aiCam.EXPECT().GetChannel().Return(uint32(1)).AnyTimes()
			aiCam.EXPECT().NvrWriteCacheToDisk(uint32(1), 2, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			// record
			aiCam.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", 0, 0, errors.New("record err")).Times(1)

			dbCli.EXPECT().UpdateEvent(uint(0), "uuid", gomock.Any()).Return(nil).Times(1)
			dbCli.EXPECT().UpdateEventVideo(uint(0), "", "", true).Return(nil).Times(1)

			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, true, time.Now(), 7)
		})

		t.Run("video to s3 err", func(t *testing.T) {
			init()
			dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).Times(1)
			dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).Times(1)

			// upload event to cloud
			b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
			b.EXPECT().UploadAICameraEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("uuid", nil).Times(1)

			// handler event video
			aiCam := mock.NewMockAICamera(ctrl)
			b.EXPECT().GetCamera(gomock.Any()).Return(aiCam, nil).AnyTimes()
			aiCam.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
			aiCam.EXPECT().GetChannel().Return(uint32(1)).AnyTimes()
			aiCam.EXPECT().NvrWriteCacheToDisk(uint32(1), 2, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			// record
			aiCam.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("path.jpg", 1920, 1080, nil).AnyTimes()

			aiCam.EXPECT().GetID().Return(10).AnyTimes()
			b.EXPECT().UploadS3ByTokenName(10, "path.jpg", 1080, 1920, "mp4", box2.TokenNameCameraEvent).Return(&utils.S3File{}, errors.New("video to s3 err")).AnyTimes()

			dbCli.EXPECT().UpdateEvent(uint(0), "uuid", gomock.Any()).Return(nil).Times(1)
			dbCli.EXPECT().UpdateEventVideo(uint(0), "path.jpg", `{"Bucket":"","Key":"","FileSize":0,"Format":"","Height":0,"Width":0}`, true).Return(nil).Times(1)

			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, true, time.Now(), 7)
		})

		t.Run("video to cloud api err", func(t *testing.T) {
			init()
			dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).Times(1)
			dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).Times(1)

			// upload event to cloud
			b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
			b.EXPECT().UploadAICameraEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("uuid", nil).Times(1)

			// handler event video
			aiCam := mock.NewMockAICamera(ctrl)
			b.EXPECT().GetCamera(gomock.Any()).Return(aiCam, nil).AnyTimes()
			aiCam.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
			aiCam.EXPECT().GetChannel().Return(uint32(1)).AnyTimes()
			aiCam.EXPECT().NvrWriteCacheToDisk(uint32(1), 2, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			// record
			aiCam.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("path.jpg", 1920, 1080, nil).AnyTimes()

			aiCam.EXPECT().GetID().Return(10).AnyTimes()
			b.EXPECT().UploadS3ByTokenName(10, "path.jpg", 1080, 1920, "mp4", box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).AnyTimes()

			b.EXPECT().UploadEventMedia("uuid", gomock.Any()).Return(errors.New("err cloud api")).AnyTimes()

			dbCli.EXPECT().UpdateEvent(uint(0), "uuid", gomock.Any()).Return(nil).Times(1)
			dbCli.EXPECT().UpdateEventVideo(uint(0), "path.jpg", `{"Bucket":"","Key":"","FileSize":0,"Format":"","Height":0,"Width":0}`, true).Return(nil).Times(1)

			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, true, time.Now(), 7)
		})

		t.Run("ok", func(t *testing.T) {
			init()
			dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).Times(1)
			dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).Times(1)

			// upload event to cloud
			b.EXPECT().UploadS3ByTokenName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
			b.EXPECT().UploadAICameraEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("uuid", nil).Times(1)

			// handler event video
			aiCam := mock.NewMockAICamera(ctrl)
			b.EXPECT().GetCamera(gomock.Any()).Return(aiCam, nil).AnyTimes()
			aiCam.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
			aiCam.EXPECT().GetChannel().Return(uint32(1)).AnyTimes()
			aiCam.EXPECT().NvrWriteCacheToDisk(uint32(1), 2, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			// record
			aiCam.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("path.jpg", 1920, 1080, nil).AnyTimes()

			aiCam.EXPECT().GetID().Return(10).AnyTimes()
			b.EXPECT().UploadS3ByTokenName(10, "path.jpg", 1080, 1920, "mp4", box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).AnyTimes()

			b.EXPECT().UploadEventMedia("uuid", gomock.Any()).Return(nil).AnyTimes()

			dbCli.EXPECT().UpdateEvent(uint(0), "uuid", gomock.Any()).Return(nil).Times(1)
			dbCli.EXPECT().UpdateEventVideo(uint(0), "", "", false).Return(nil).Times(1)

			u.processEvent(univCam, EventCar, time.Now().Unix(), data, &structs.MetaScanData{}, true, true, true, time.Now(), 7)
		})
	})
}

func TestUniviewAPI_parsePosition(t *testing.T) {
	type fields struct {
		Box    box2.Box
		DB     db.Client
		Logger zerolog.Logger
	}
	type args struct {
		position string
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantRect structs.Rectangle
		wantErr  bool
	}{
		{
			name: "coordinates test, legal format (100,200;300,400)",
			fields: fields{
				Box:    nil,
				DB:     nil,
				Logger: zerolog.Logger{},
			},
			args: args{position: "100,200;300,400"},
			wantRect: structs.Rectangle{
				X:      100,
				Y:      200,
				Width:  200,
				Height: 200,
			},
			wantErr: false,
		},
		{
			name: "coordinates test, illegal format (100,200,300,400)",
			fields: fields{
				Box:    nil,
				DB:     nil,
				Logger: zerolog.Logger{},
			},
			args: args{position: "100,200,300,400"},
			wantRect: structs.Rectangle{
				X:      0,
				Y:      0,
				Width:  0,
				Height: 0,
			},
			wantErr: true,
		},
		{
			name: "coordinates test, illegal format (100,200,300;400)",
			fields: fields{
				Box:    nil,
				DB:     nil,
				Logger: zerolog.Logger{},
			},
			args: args{position: "100,200,300;400"},
			wantRect: structs.Rectangle{
				X:      0,
				Y:      0,
				Width:  0,
				Height: 0,
			},
			wantErr: true,
		},
		{
			name: "coordinates test, illegal format (abcd;300,400)",
			fields: fields{
				Box:    nil,
				DB:     nil,
				Logger: zerolog.Logger{},
			},
			args: args{position: "abcd;300,400"},
			wantRect: structs.Rectangle{
				X:      0,
				Y:      0,
				Width:  0,
				Height: 0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &UniviewAPI{
				Box:    tt.fields.Box,
				DB:     tt.fields.DB,
				Logger: tt.fields.Logger,
			}
			gotRect, err := u.parsePosition(tt.args.position)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePosition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRect, tt.wantRect) {
				t.Errorf("parsePosition() gotRect = %v, want %v", gotRect, tt.wantRect)
			}
		})
	}
}

func TestUniviewAPI_assembleEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	cfg := configs.NewEmptyConfig()
	c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
	univCam := c.(*uniview.BaseUniviewCamera)
	box.EXPECT().GetConfig().Return(&cfg).AnyTimes()
	dbCli := mock.NewMockDBClient(ctrl)
	u := getMockUniviewApi(box, dbCli)
	imageList := []ImageInfo{
		{
			Data: data,
		},
		{
			Data: data,
		},
	}
	objectInfoList := []ObjectDetected{
		{
			Position:            "100,200,300,400",
			LargePicAttachIndex: 2,
		},
		{
			Position:            "100,200;300,400",
			LargePicAttachIndex: 2,
		},
	}
	cli2, _ := db.NewDBClient(&cfg, "file::memory:")
	dbCli.EXPECT().GetDBInstance().Return(cli2.GetDBInstance())
	u.assembleEvent(univCam, "", EventCar, 10, false, false, false, imageList, objectInfoList, time.Now(), 7)
}

func TestUniviewAPI_handlerLocalDetectEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	cfg := configs.NewEmptyConfig()
	c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
	univCam := c.(*uniview.BaseUniviewCamera)
	univCam.DetectParams = model.DetectParams{
		DetectAt:            "box",
		CarThreshold:        0.2,
		PersonThreshold:     0.2,
		MotorcycleThreshold: 0.2,
	}
	box.EXPECT().GetConfig().Return(&cfg).AnyTimes()
	dbCli := mock.NewMockDBClient(ctrl)
	imageList := []ImageInfo{
		{
			Data: data,
		},
		{
			Data: data,
		},
	}

	t.Run("null image list", func(t *testing.T) {
		u := getMockUniviewApi(box, dbCli)
		err := u.handlerLocalDetectEvent(univCam, time.Now().Unix(), []ImageInfo{}, false, false, false, time.Now(), 7)
		assert.Error(t, err)
	})

	t.Run("ObjectDetect error", func(t *testing.T) {
		box.EXPECT().ObjectDetect(gomock.Any()).Return(mockError, nil).Times(2)
		u := getMockUniviewApi(box, dbCli)
		err := u.handlerLocalDetectEvent(univCam, time.Now().Unix(), imageList, false, false, false, time.Now(), 7)
		assert.Nil(t, err)
	})

	t.Run("ObjectDetect objects is null", func(t *testing.T) {
		object := []utils.DetectObjects{
			{},
		}
		box.EXPECT().ObjectDetect(gomock.Any()).Return(nil, object).Times(2)
		u := getMockUniviewApi(box, dbCli)
		err := u.handlerLocalDetectEvent(univCam, time.Now().Unix(), imageList, false, false, false, time.Now(), 7)
		assert.Nil(t, err)
	})

}

func TestUniviewAPI_saveEventToDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)

	t.Run("DecodeBase64Image error", func(t *testing.T) {
		t.Parallel()
		dbCli := mock.NewMockDBClient(ctrl)
		u := getMockUniviewApi(box, dbCli)
		dbCli.EXPECT().SavePictureToDB(&db.FaceScan{Rect: db.FaceRect{}, ImgBase64: "base64", Format: "JPEG"}).Return(db.FacePicture{}, errors.New("not nil"))
		id, err := u.saveEventToDB("test_sn", 20, EventCar, "base64", &structs.MetaScanData{}, time.Now().Unix(), time.Now())
		assert.NotNil(t, err)
		assert.EqualValues(t, id, 0)
	})

	t.Run("SavePictureToDB error", func(t *testing.T) {
		t.Parallel()
		dbCli := mock.NewMockDBClient(ctrl)
		dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{}, mockError).AnyTimes()
		u := getMockUniviewApi(box, dbCli)
		id, err := u.saveEventToDB("test_sn", 20, EventCar, data, &structs.MetaScanData{}, time.Now().Unix(), time.Now())
		assert.Equal(t, err, mockError)
		assert.EqualValues(t, id, 0)
	})

	t.Run("CreateEvent error", func(t *testing.T) {
		t.Parallel()
		dbCli := mock.NewMockDBClient(ctrl)
		dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).AnyTimes()
		dbCli.EXPECT().CreateEvent(gomock.Any()).Return(mockError).AnyTimes()
		u := getMockUniviewApi(box, dbCli)
		id, err := u.saveEventToDB("test_sn", 20, EventCar, data, &structs.MetaScanData{}, time.Now().Unix(), time.Now())
		assert.Equal(t, err, mockError)
		assert.EqualValues(t, id, 0)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		dbCli := mock.NewMockDBClient(ctrl)
		dbCli.EXPECT().SavePictureToDB(gomock.Any()).Return(db.FacePicture{ID: 1}, nil).AnyTimes()
		dbCli.EXPECT().CreateEvent(gomock.Any()).Return(nil).AnyTimes()
		u := getMockUniviewApi(box, dbCli)
		id, err := u.saveEventToDB("test_sn", 20, EventCar, data, &structs.MetaScanData{}, time.Now().Unix(), time.Now())
		assert.Nil(t, err)
		assert.EqualValues(t, id, 0)
	})

}

func TestUniviewApi_uploadEventToCloud(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)
	cfg := configs.NewEmptyConfig()
	box.EXPECT().GetConfig().Return(&cfg).AnyTimes()
	box.EXPECT().UploadS3ByTokenName(10, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).AnyTimes()
	box.EXPECT().UploadS3ByTokenName(11, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, mockError).AnyTimes()
	box.EXPECT().UploadS3ByTokenName(12, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).AnyTimes()
	box.EXPECT().UploadAICameraEvent(10, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("123", nil).AnyTimes()
	box.EXPECT().UploadAICameraEvent(12, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", mockError).AnyTimes()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		u := getMockUniviewApi(box, db)
		id, err := u.uploadEventToCloud(data, 10, EventCar, time.Now(), time.Now(), &structs.MetaScanData{})
		assert.Nil(t, err)
		assert.EqualValues(t, id, "123")
	})

	t.Run("uploadFileToS3 error", func(t *testing.T) {
		t.Parallel()
		u := getMockUniviewApi(box, db)
		id, err := u.uploadEventToCloud(data, 11, EventCar, time.Now(), time.Now(), &structs.MetaScanData{})
		assert.Equal(t, err, mockError)
		assert.EqualValues(t, id, "")
	})

	t.Run("UploadAICameraEvent error", func(t *testing.T) {
		t.Parallel()
		u := getMockUniviewApi(box, db)
		id, err := u.uploadEventToCloud(data, 12, EventCar, time.Now(), time.Now(), &structs.MetaScanData{})
		assert.Equal(t, err, mockError)
		assert.EqualValues(t, id, "")
	})
}

func TestUniviewAPI_convertCoordinates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)
	u := getMockUniviewApi(box, db)
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mockW := 1000
		mockH := 1000
		mockMeta := &structs.MetaScanData{
			Objects: []structs.ObjectInfo{
				{
					BBox: structs.Rectangle{X: 10, Y: 20, Width: 120, Height: 240},
				},
			},
			PolygonInfos: []structs.PolygonInfo{
				{
					Points: []structs.Point{{X: 0, Y: 0}, {X: 100, Y: 200}, {X: 600, Y: 800}},
				},
			},
		}
		expectedMeta := &structs.MetaScanData{
			Objects: []structs.ObjectInfo{
				{
					BBox: structs.Rectangle{X: 1, Y: 2, Width: 12, Height: 24},
				},
			},
			PolygonInfos: []structs.PolygonInfo{
				{
					Points: []structs.Point{{X: 0, Y: 0}, {X: 10, Y: 20}, {X: 60, Y: 80}},
				},
			},
		}
		t.Log(mockMeta)
		u.convertCoordinates(mockW, mockH, mockMeta)
		assert.EqualValues(t, expectedMeta, mockMeta)
		t.Log(mockMeta)
	})
}

func TestUniviewAPI_uploadFileToS3(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	dbCli := mock.NewMockDBClient(ctrl)
	cfg := configs.NewEmptyConfig()
	box.EXPECT().GetConfig().Return(&cfg).AnyTimes()
	box.EXPECT().UploadS3ByTokenName(10, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).AnyTimes()
	box.EXPECT().UploadS3ByTokenName(11, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(nil, mockError).AnyTimes()
	t.Run("SaveImage error", func(t *testing.T) {
		t.Parallel()
		u := getMockUniviewApi(box, dbCli)
		s3File, err := u.uploadFileToS3(10, &db.FaceScan{ImgBase64: "img"})
		assert.NotNil(t, err)
		assert.Nil(t, s3File)
	})
	t.Run("UploadS3ByTokenName error", func(t *testing.T) {
		t.Parallel()
		u := getMockUniviewApi(box, dbCli)
		s3File, err := u.uploadFileToS3(11, &db.FaceScan{ImgBase64: data})
		assert.Equal(t, err, mockError)
		assert.Nil(t, s3File)
	})
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		u := getMockUniviewApi(box, dbCli)
		s3File, err := u.uploadFileToS3(10, &db.FaceScan{ImgBase64: data})
		assert.Nil(t, err)
		assert.NotNil(t, s3File)
	})
}

func TestUniviewAPI_handleEventVideo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	cfg := configs.NewEmptyConfig()
	c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
	univCam := c.(*uniview.BaseUniviewCamera)
	box.EXPECT().GetConfig().Return(&cfg).AnyTimes()
	dbCli := mock.NewMockDBClient(ctrl)

	t.Run("GetCamera error", func(t *testing.T) {
		box.EXPECT().GetCamera(11).Return(univCam, mockError)
		u := getMockUniviewApi(box, dbCli)
		u.handleEventVideo("111", 11, 10, 20)
	})

	t.Run("Record error", func(t *testing.T) {
		aiCam := mock.NewMockAICamera(ctrl)
		aiCam.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("path.jpg", 1920, 1080, mockError).AnyTimes()
		aiCam.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
		aiCam.EXPECT().GetChannel().Return(uint32(1)).AnyTimes()
		aiCam.EXPECT().NvrWriteCacheToDisk(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		box.EXPECT().GetCamera(13).Return(aiCam, nil).AnyTimes()
		u := getMockUniviewApi(box, dbCli)
		u.handleEventVideo("111", 13, 10, 20)
	})

	t.Run("UploadEventMedia resolution", func(t *testing.T) {
		camID := 14
		ids := []int{utils.HD_ID, utils.Normal_ID, utils.SD_ID}
		times := len(ids)
		aiCam := mock.NewMockAICamera(ctrl)
		aiCam.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("path.jpg", 0, 0, nil).Times(times)
		aiCam.EXPECT().GetID().Return(14).AnyTimes()
		aiCam.EXPECT().GetChannel().Return(uint32(1)).Times(times)
		aiCam.EXPECT().NvrWriteCacheToDisk(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(times)
		box.EXPECT().GetCamera(14).Return(aiCam, nil).Times(times)

		for id := range ids {
			info := univiewapi.VideoStreamInfo{}
			info.ID = uint32(id)
			info.VideoEncodeInfo.Resolution.Width = 1280
			info.VideoEncodeInfo.Resolution.Height = 720

			if utils.HD_ID == id {
				info.VideoEncodeInfo.EncodeFormat = utils.CodecTypeH265Index
				aiCam.EXPECT().GetManufacturer().Return(utils.TuringSunell).Times(1)
			} else if utils.Normal_ID == id {
				info.VideoEncodeInfo.EncodeFormat = utils.CodecTypeH264Index
				aiCam.EXPECT().GetManufacturer().Return(utils.TuringUniview).Times(1)
			} else if utils.SD_ID == id {
				info.VideoEncodeInfo.EncodeFormat = utils.CodecTypeMjpegIndex
				aiCam.EXPECT().GetManufacturer().Return(utils.TuringUniview).Times(1)
			}
			codecType := utils.GetCodecTypeIdByID(info.VideoEncodeInfo.EncodeFormat)

			cloud.ClearCameraSettings()
			cloud.SaveCameraSettings([]*cloud.CameraSettings{
				{
					CamID:          camID,
					StreamSettings: []univiewapi.VideoStreamInfo{info},
				},
			})

			var startTime int64 = 10
			var endTime int64 = 20
			s3File := utils.S3File{
				Height: int(info.VideoEncodeInfo.Resolution.Height),
				Width:  int(info.VideoEncodeInfo.Resolution.Width),
			}

			var videos []cloud.MediaVideo
			videos = append(videos, cloud.MediaVideo{
				File: cloud.File{
					Meta: cloud.Meta{
						Size:        []int{s3File.Height, s3File.Width},
						ContentType: "video/" + s3File.Format,
						CodecType:   codecType,
					},
				},
				StartedAt: time.Unix(startTime, 0).Format(utils.CloudTimeLayout),
				EndedAt:   time.Unix(endTime, 0).Format(utils.CloudTimeLayout),
			})

			if utils.SD_ID != id {
				box.EXPECT().UploadS3ByTokenName(camID, gomock.Any(), int(info.VideoEncodeInfo.Resolution.Height), int(info.VideoEncodeInfo.Resolution.Width),
					gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{Height: s3File.Height, Width: s3File.Width}, nil).Times(1)
				box.EXPECT().UploadEventMedia(gomock.Any(), &cloud.Media{
					Videos: &videos,
				}).Return(nil).Times(1)
			} else {
				box.EXPECT().UploadS3ByTokenName(camID, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).Times(1)
				box.EXPECT().UploadEventMedia(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			}

			u := getMockUniviewApi(box, dbCli)
			u.handleEventVideo("111", camID, startTime, endTime)
		}
	})

	aiCam := mock.NewMockAICamera(ctrl)
	aiCam.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("path.jpg", 1920, 1080, nil).AnyTimes()
	aiCam.EXPECT().GetID().Return(14).AnyTimes()
	aiCam.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
	aiCam.EXPECT().GetChannel().Return(uint32(1)).AnyTimes()
	aiCam.EXPECT().NvrWriteCacheToDisk(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	box.EXPECT().GetCamera(14).Return(aiCam, nil).AnyTimes()

	t.Run("UploadS3ByTokenName error", func(t *testing.T) {
		box.EXPECT().UploadS3ByTokenName(14, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, mockError).Times(3)
		u := getMockUniviewApi(box, dbCli)
		u.handleEventVideo("111", 14, 10, 20)
	})

	box.EXPECT().UploadS3ByTokenName(14, gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil).AnyTimes()
	t.Run("UploadEventMedia error", func(t *testing.T) {
		box.EXPECT().UploadEventMedia(gomock.Any(), gomock.Any()).Return(mockError).AnyTimes()

		u := getMockUniviewApi(box, dbCli)
		u.handleEventVideo("111", 14, 10, 20)
	})

	t.Run("UploadEventMedia success", func(t *testing.T) {
		box.EXPECT().UploadEventMedia(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		u := getMockUniviewApi(box, dbCli)
		u.handleEventVideo("111", 14, 10, 20)
	})
}

func TestUniviewAPI_getPolygonInfos(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	cfg := configs.NewEmptyConfig()
	c := uniview.NewCamera(&cloud.Camera{ID: 10, Uri: "rtsp://192.168.11.128/unicast/c1/s0/live"}, &cfg)
	univCam := c.(*uniview.BaseUniviewCamera)
	box.EXPECT().GetConfig().Return(&cfg).AnyTimes()
	dbCli := mock.NewMockDBClient(ctrl)
	cli2, _ := db.NewDBClient(&cfg, "file::memory:")
	dbCli.EXPECT().GetDBInstance().Return(cli2.GetDBInstance()).Times(2)

	t.Run("UploadEventMedia success", func(t *testing.T) {
		u := getMockUniviewApi(box, dbCli)
		polys := u.getPolygonInfos(univCam, EventEnterArea, cloud.Intrude)
		t.Log(polys)
		polys = u.getPolygonInfos(univCam, EventIntrusion, cloud.Car)
		t.Log(polys)
	})
}

func Test_getPolygonFromRule(t *testing.T) {
	type args struct {
		data string
	}
	tests := []struct {
		name string
		args args
		want []structs.PolygonInfo
	}{
		{
			name: "null",
			args: args{
				data: "",
			},
			want: []structs.PolygonInfo{},
		},
		{
			name: "illegal json",
			args: args{
				data: "{aaa}",
			},
			want: []structs.PolygonInfo{},
		},
		{
			name: "no point",
			args: args{
				data: `{"Num":4,"PolygonInfoList":[{"ID":0,"Enabled":1,"Sensitivity":83,"Percentage":0,"TimeThreshold":0, "PointNum":0,"PointList":[],"Priority":0}`,
			},
			want: []structs.PolygonInfo{},
		},
		{
			name: "normal",
			args: args{
				data: `{"Num":4,"PolygonInfoList":[{"ID":0,"Enabled":1,"Sensitivity":83,"Percentage":0,"TimeThreshold":0,
	"PointNum":4,"PointList":[{"X":197,"Y":9354},{"X":9549,"Y":9612},{"X":9802,"Y":290},{"X":366,"Y":225}],"Priority":0,
	"Num":3,"DetectTargetList":[{"Enabled":1,"Type":0,"ObjectMaxSize":{"Width":10000,"Height":10000},"ObjectMinSize":{
	"Width":105,"Height":186}},{"Enabled":1,"Type":1,"ObjectMaxSize":{"Width":10000,"Height":10000},"ObjectMinSize":{
	"Width":105,"Height":186}},{"Enabled":1,"Type":2,"ObjectMaxSize":{"Width":10000,"Height":10000},"ObjectMinSize":{
	"Width":105,"Height":186}}]}]}`,
			},
			want: []structs.PolygonInfo{
				{
					Points:       []structs.Point{{X: 197, Y: 9354}, {X: 9549, Y: 9612}, {X: 9802, Y: 290}, {X: 366, Y: 225}},
					EnabledTypes: []int{0, 1, 2},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPolygonFromRule(tt.args.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPolygonFromRule() = %v, want %v", got, tt.want)
			}
		})
	}
}
