package box

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/example/goshawk/uniview"
	"github.com/example/turing-common/model"

	"github.com/example/minibox/apis/structs"
	"github.com/example/minibox/camera/base"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/configs"
	"github.com/example/minibox/db"
	"github.com/example/minibox/discover/arp"
	"github.com/example/minibox/mock"
	"github.com/example/minibox/utils"
)

const testPhoto = "iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAFKElEQVRYhcWX608UZxTG919o+qHfqk1ra0xjYlubtdQqDY0x6iK6ChZ0FYT1Urx010W8Fos2TdAKWLW0VqxJFWLrwmI1tlguiY1WoyVURS7LzLwzoxixKAsoMr9+WGdAd61gjUxykknmXJ73nPOc845NVVUkSUKWZXRdR9M0hBCoqoqqqsiybH0TQhAMBtE0DUVR0HUdVVUtGyEEkiShKArBYND6HgwGURTF8mf6VlUVmyzLCCGQZdkyHGjc2trKicpT5OzIJyNrHfGpS4hLWkBc0gLsDid2h5MPE+cTl7SAaa7FuNeuJ2dHAccrf0d5cDgTnKqqKIpiiSzL2ILBICeranA/cD5/hZfvD5UgSRKlZQEcqRlWoKHKzDQ3h372I0kSh46WsXC1jxmL0knzrMX/y4lwBo6drCQmITHCeNrCxdZ7TKKL+NwCkg9U4K6sJ/O8zupLHXhb7uFtucfqSx1kntdxV9aTfKCC+NwCYhJdln1C2tII/xPi51DiL8eW6s3G7nAy5dNNJBeXM2NTnqU0ybUE15EqvE138UnGkMTb1IPrSBWTXO7wIebMZ07BQRK/KeWD+enYHU6cGcuxzViUzuTU5Xhb7rGmORwoNXAGx+cFZDeGqLhl0NwN6l2oC0FxW/SAWZLBwRsGf9yBc50QaDf4TDHwNIRw5h9gxYXreBq7WVV3k6W1jeEDOj/G5s7awFTfFjxX7jB7+z5m7diH52oXuUofbfeI+vzZCVkDgucoBq09kXpdfVB03cDXauA+dYmEL/cyb38ZnoYQdoeTpGWrsJ2orOK9hCQyfq1jVd1NfMH7+Fr7ojoc+ATa+wE0dD9er6cPtqn9up7Gbpz5P2B3ODl8tAybqqqUlgWInbcAR85OPjkjs+eaEeEoFArhSklmzOujyM7y0dEbTnuhHqlbV/cXkyfGMG7sm5T7/dTcBp9ksKTqCh9lrmXirCSKD5cihMBmcjRz0xbsDidpx85ScSvS6d7du3npxRcsqa6q4gvVoKw9Unf2zHhL77WRL9PU2Wv1lt3hJHNjDpqmoes6NkVREEKQsDhMlZUXbxCI4rRo756HANTWVLPtMQCcCTMtvVGvjKA5dB+fZLDyYht2h5NZ6Uv7B5EJIHZuCnaHE29TD19HKUFXVxdpi1yMHTOazRvXc+tBCfKjlODv+nriYifz7lvjOFYRoLoDi5p2h5PYuSnW2LapqooQwuK+2SwtT2hC355i3pkynfHTEij6KfBYve4+2Cr6m9CMo+t6OAPmDngUQK4wuBaVhgaVZ8/x6sgRlowZ/QaXW5ojNDv7YO+1h+fFQACKooRLEA2ATzJYL4dr3NANyl24EIJjJcXU5nnZmjqbqRPeZvr749m5bB61eV7KK/zU3oYzd+Bou8FmJXJgmXHMDWszUxENwKOS3dRD1XYftXneqHL8qw1PHNFmHHOd28xVPBgAa1p6WVhUwobCXRzfv4vqH4uoOfQt/u8KWVe4G9d+/6ABWPcBcw4MBoBPMvA09bD8nEbbPx1wvxfu93JZvc6yswJPU8+gAVgl0DQNSZIGDcCU85JmAfjtqjxoOzOOEKJ/ED2uCf9L0quvkuI/TYr/NEtOB4cMQNM0NE0LZ0DX9SEDeFoZ2ISKooRZoKrqcwdgNaF5ax0OAEIIbJIkoWnacwdgXtWtZfS8AZi/A0OeA8+yBJqmhf8LnoaG/xeAJEn9k3A4WGCyz2YuheGYA0KIMIDhYIHVA+ZAGA4aSpI0tPvAs86ALMv8CysqfF+oqf78AAAAAElFTkSuQmCC"

func TestUploadCameraEvent(t *testing.T) {
	t.Run("client is nil", func(t *testing.T) {
		t.Parallel()
		cfg := configs.NewEmptyConfig()
		b := New(&cfg, configs.BoxInfo{}, nil)

		_, err := b.UploadCameraEvent(0, nil, 0, 0, nil, time.Now())
		assert.Equal(t, ErrNoAPIClient, err)
	})

	t.Run("algoMap error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client := mock.NewMockClient(ctrl)
		b := baseBox{
			algoMap:   &sync.Map{},
			apiClient: client,
		}

		id, err := b.UploadCameraEvent(0, nil, 0, 0, &structs.MetaScanData{}, time.Now())
		assert.NotNil(t, err)
		assert.Zero(t, id)
	})

	t.Run("upload the event", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		algoMap := &sync.Map{}
		algoMap.Store(EventTemperatureNormal, "temperature_normal:code")
		algoMap.Store(EventTemperatureAbnormal, "temperature_abnormal:code")
		algoMap.Store(EventQuestionnaireFail, "questionnaire_fail:code")
		client := mock.NewMockClient(ctrl)
		b := baseBox{
			algoMap:   algoMap,
			apiClient: client,
		}

		client.EXPECT().UploadCameraEvent(gomock.Any()).Return(nil, errors.New("fail"))

		now := time.Now()
		id, err := b.UploadCameraEvent(0, nil, 0, 0, nil, now)
		assert.NotNil(t, ErrArgsWrongType)
		assert.Zero(t, id)

		client.EXPECT().UploadCameraEvent(gomock.Eq(&cloud.CameraEvent{
			StartedAt:   now.UTC(),
			Algos:       []string{"temperature_normal:code"},
			Temperature: &cloud.TemperatureInfo{},
		})).Return(&cloud.Event{ID: "1"}, nil)

		id, err = b.UploadCameraEvent(0, nil, 0, 0, nil, now)
		assert.Nil(t, err)
		assert.Equal(t, "1", id)

		client.EXPECT().UploadCameraEvent(gomock.Eq(&cloud.CameraEvent{
			StartedAt: now.UTC(),
			Algos:     []string{"temperature_abnormal:code"},
			Temperature: &cloud.TemperatureInfo{
				Temperature: 30.0,
			},
		})).Return(&cloud.Event{ID: "2"}, nil)

		id, err = b.UploadCameraEvent(0, nil, 30.0, 25.0, nil, now)
		assert.Nil(t, err)
		assert.Equal(t, "2", id)

		meta := &structs.MetaScanData{
			QuestionnaireResult: false,
			HasQuestionnaire:    true,
		}
		client.EXPECT().UploadCameraEvent(gomock.Eq(&cloud.CameraEvent{
			StartedAt: now.UTC(),
			Algos:     []string{"questionnaire_fail:code"},
			Temperature: &cloud.TemperatureInfo{
				Temperature: 30.0,
			},
			MetaScanData: meta,
		})).Return(&cloud.Event{ID: "3"}, nil)

		id, err = b.UploadCameraEvent(0, nil, 30.0, 25.0, meta, now)
		assert.Nil(t, err)
		assert.Equal(t, "3", id)
	})
}

func TestUploadCameraState(t *testing.T) {
	t.Run("client is nil", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := mock.NewMockConfig(ctrl)
		cameraSet := mock.NewMockCamGroup(ctrl)
		b := baseBox{
			camGroup:  cameraSet,
			config:    cfg,
			configMux: &sync.Mutex{},
		}

		wg := sync.WaitGroup{}
		wg.Add(1)

		cfg.EXPECT().GetHeartBeatSecs().Return(1)
		cameraSet.EXPECT().GetAllCameraStatus().Return(map[int]cloud.CameraState{1: {CameraID: 1, State: "running"}})
		cfg.EXPECT().GetCameraRefreshMins().Do(func() {
			wg.Done()
		}).Return(1)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go b.uploadCameraState(ctx)

		assert.False(t, utils.WaitTimeout(&wg, 1*2*time.Second))
	})

	t.Run("no camera state", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := mock.NewMockConfig(ctrl)
		cameraSet := mock.NewMockCamGroup(ctrl)
		client := mock.NewMockClient(ctrl)
		b := baseBox{
			camGroup:  cameraSet,
			config:    cfg,
			configMux: &sync.Mutex{},
			apiClient: client,
		}

		wg := sync.WaitGroup{}
		wg.Add(1)

		cfg.EXPECT().GetHeartBeatSecs().Return(1)
		cameraSet.EXPECT().GetAllCameraStatus().
			Do(func() {
				wg.Done()
			}).
			Return(map[int]cloud.CameraState{})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go b.uploadCameraState(ctx)

		assert.False(t, utils.WaitTimeout(&wg, 2*time.Second))
	})

	t.Run("camera state the same", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := mock.NewMockConfig(ctrl)
		cameraSet := mock.NewMockCamGroup(ctrl)
		client := mock.NewMockClient(ctrl)
		nvrManager := mock.NewMockNVRManager(ctrl)
		camera := mock.NewMockAICamera(ctrl)
		b := baseBox{
			camGroup:   cameraSet,
			config:     cfg,
			configMux:  &sync.Mutex{},
			apiClient:  client,
			nvrManager: nvrManager,
		}

		wg := sync.WaitGroup{}
		wg.Add(3)

		status := map[int]cloud.CameraState{
			1: {
				CameraID: 1,
				State:    model.CameraStateRunning,
			},
			2: {
				CameraID: 2,
				State:    model.CameraStateOffline,
			},
		}
		cfg.EXPECT().GetHeartBeatSecs().Return(1)
		cameraSet.EXPECT().GetAllCameraStatus().Return(status).Times(2)
		cameraSet.EXPECT().GetCamera(gomock.Any()).Return(camera, nil).Times(2)
		nvrManager.EXPECT().RefreshDevice().Return().Times(2)
		cfg.EXPECT().GetCameraRefreshMins().Do(func() {
			wg.Done()
		}).Return(1).Times(2)

		client.EXPECT().UploadCameraState(gomock.Any()).Do(func(arg0 interface{}) {
			wg.Done()
		}).Return(nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go b.uploadCameraState(ctx)

		// normal exit
		assert.False(t, utils.WaitTimeout(&wg, 3*time.Second))
	})

	t.Run("camera state error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := mock.NewMockConfig(ctrl)
		cameraSet := mock.NewMockCamGroup(ctrl)
		client := mock.NewMockClient(ctrl)
		b := baseBox{
			camGroup:  cameraSet,
			config:    cfg,
			configMux: &sync.Mutex{},
			apiClient: client,
		}

		wg := sync.WaitGroup{}
		wg.Add(2)

		status := map[int]cloud.CameraState{1: {CameraID: 1, State: "running"}}

		cfg.EXPECT().GetHeartBeatSecs().Return(1)
		cfg.EXPECT().GetCameraRefreshMins().Return(1).Times(2)

		cameraSet.EXPECT().GetAllCameraStatus().Return(status).Times(2)

		client.EXPECT().UploadCameraState(gomock.Eq(status)).Do(func(arg0 interface{}) {
			wg.Done()
		}).Return(errors.New("err")).Times(2)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go b.uploadCameraState(ctx)

		assert.False(t, utils.WaitTimeout(&wg, 4*time.Second))
	})

	t.Run("camera state change once", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := mock.NewMockConfig(ctrl)
		cameraSet := mock.NewMockCamGroup(ctrl)
		client := mock.NewMockClient(ctrl)
		b := baseBox{
			camGroup:  cameraSet,
			config:    cfg,
			configMux: &sync.Mutex{},
			apiClient: client,
		}

		wg := sync.WaitGroup{}
		wg.Add(3)

		status := map[int]cloud.CameraState{1: {CameraID: 1, State: "running"}}

		cfg.EXPECT().GetHeartBeatSecs().Return(1)
		cfg.EXPECT().GetCameraRefreshMins().Return(1).Times(2)

		cameraSet.EXPECT().GetAllCameraStatus().Do(func() {
			wg.Done()
		}).Return(status).Times(2)
		client.EXPECT().UploadCameraState(gomock.Eq(status)).Do(func(arg0 interface{}) {
			wg.Done()
		}).Return(nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go b.uploadCameraState(ctx)

		assert.False(t, utils.WaitTimeout(&wg, 3*time.Second))
	})
}

func TestCloudClient(t *testing.T) {
	u, _ := url.Parse("http://google.com")
	c := cloud.NewClient(cloud.ClientConfig{
		CloudServerURL: u,
	})
	b := baseBox{
		apiClient: c,
	}

	assert.Equal(t, c, b.CloudClient())
}

func TestHandleUploadEvents(t *testing.T) {
	t.Run("fail update camera ID", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
		}

		data.EXPECT().GetEventsWithoutCameraID().Return(nil, errors.New("err"))

		b.handleRetryUploadEvents()
	})

	t.Run("fail get cameras", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			apiClient: client,
		}

		data.EXPECT().GetEventsWithoutCameraID().Return([]db.Event{
			db.Event{
				CameraSN: "test_sn",
			},
		}, nil)

		client.EXPECT().GetCameras().Return(nil, errors.New("err"))
		b.handleRetryUploadEvents()
	})

	t.Run("fail get events with no remote ID", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			apiClient: client,
		}

		data.EXPECT().GetEventsWithoutCameraID().Return([]db.Event{
			db.Event{
				ID:       2,
				CameraSN: "test_sn",
			},
		}, nil)
		data.EXPECT().UpdateEvent(gomock.Eq(uint(2)), gomock.Eq(""), gomock.Eq(int64(1))).Return(nil)
		data.EXPECT().GetRetryEvents().Return(nil, errors.New("err"))

		client.EXPECT().GetCameras().Return([]*cloud.Camera{
			&cloud.Camera{
				SN: "test_sn",
				ID: 1,
			},
		}, nil)

		b.handleRetryUploadEvents()
	})

	t.Run("no events", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			apiClient: client,
		}

		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return(nil, nil)

		b.handleRetryUploadEvents()
	})

	t.Run("event created in 5 min", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			apiClient: client,
			camGroup:  camGroup,
		}
		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				CameraID:  1,
				PictureID: 1,
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-2 * time.Minute),
			},
		}, nil)
		b.handleRetryUploadEvents()
	})

	t.Run("hour diff greater than event save hours", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			apiClient: client,
			camGroup:  camGroup,
		}
		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				CameraID:  1,
				PictureID: 1,
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		}, nil)
		_ = cfg.SetEventSavedHours(1)
		data.EXPECT().EventStopRetry(gomock.Any()).Return(nil)

		b.handleRetryUploadEvents()
	})

	t.Run("retry count greater than event retry count in 1 hour", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			apiClient: client,
			camGroup:  camGroup,
		}
		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				CameraID:   1,
				PictureID:  1,
				Type:       EventTemperatureNormal,
				CreatedAt:  time.Now().Add(-10 * time.Minute),
				RetryCount: 6,
			},
		}, nil)
		_ = cfg.SetEventSavedHours(1)
		_ = cfg.SetEventRetryCount(5)

		b.handleRetryUploadEvents()
	})

	t.Run("get camera err", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			apiClient: client,
			camGroup:  camGroup,
		}
		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				CameraID:  1,
				PictureID: 1,
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)
		_ = cfg.SetEventSavedHours(1)
		camGroup.EXPECT().GetCamera(gomock.Any()).Return(nil, errors.New("err get camera"))

		b.handleRetryUploadEvents()
	})

	t.Run("get face picture error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			apiClient: client,
			camGroup:  camGroup,
		}
		camGroup.EXPECT().GetCamera(gomock.Any()).Return(mock.NewMockThermal1Camera(ctrl), nil)
		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				CameraID:  1,
				PictureID: 1,
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)
		_ = cfg.SetEventSavedHours(1)
		data.EXPECT().IncrementRetryEvent(gomock.Any()).Return(nil).AnyTimes()
		data.EXPECT().GetFacePicture(gomock.Eq(uint(1))).Return(nil, errors.New("err"))

		b.handleRetryUploadEvents()
	})

	t.Run("get data error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			apiClient: client,
			camGroup:  camGroup,
		}

		camGroup.EXPECT().GetCamera(gomock.Any()).Return(mock.NewMockThermal1Camera(ctrl), nil)
		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				PictureID: 1,
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)
		_ = cfg.SetEventSavedHours(1)
		data.EXPECT().IncrementRetryEvent(gomock.Any()).Return(nil).AnyTimes()
		data.EXPECT().GetFacePicture(gomock.Eq(uint(1))).Return(&db.FacePicture{}, nil)

		b.handleRetryUploadEvents()
	})

	t.Run("save image error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewTestConfig(nil, configs.LogConfig{}, configs.RedisConfig{}, configs.BoxConfig{
			EventRetryCount: 5,
			EventSavedHours: 1,
			StoreDir:        os.TempDir(),
		})
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			apiClient: client,
			logger:    zerolog.New(zerolog.NewConsoleWriter()),
			camGroup:  camGroup,
		}
		scan := structs.MetaScanData{
			Temperature: 30.0,
		}

		bt, _ := json.Marshal(scan)

		camGroup.EXPECT().GetCamera(gomock.Any()).Return(mock.NewMockThermal1Camera(ctrl), nil)
		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				PictureID: 1,
				Data:      strconv.Quote(string(bt)),
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)
		data.EXPECT().IncrementRetryEvent(gomock.Any()).Return(nil).AnyTimes()
		data.EXPECT().GetFacePicture(gomock.Eq(uint(1))).Return(&db.FacePicture{}, nil)

		b.handleRetryUploadEvents()
	})

	t.Run("get token error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewTestConfig(nil, configs.LogConfig{}, configs.RedisConfig{}, configs.BoxConfig{
			EventRetryCount: 5,
			EventSavedHours: 1,
			StoreDir:        os.TempDir(),
		})
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:     &cfg,
			configMux:  &sync.Mutex{},
			db:         data,
			apiClient:  client,
			logger:     zerolog.New(zerolog.NewConsoleWriter()),
			camGroup:   camGroup,
			mux:        &sync.Mutex{},
			tokenCache: &sync.Map{},
		}
		scan := structs.MetaScanData{
			Temperature: 30.0,
		}

		bt, _ := json.Marshal(scan)

		camGroup.EXPECT().GetCamera(gomock.Any()).Return(mock.NewMockThermal1Camera(ctrl), nil)
		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				PictureID: 1,
				Data:      strconv.Quote(string(bt)),
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
				CameraID:  1,
			},
		}, nil)
		data.EXPECT().IncrementRetryEvent(gomock.Any()).Return(nil).AnyTimes()
		data.EXPECT().GetFacePicture(gomock.Eq(uint(1))).Return(&db.FacePicture{
			Data: []byte(testPhoto),
		}, nil)

		client.EXPECT().GetTokenByCamera(gomock.Any(), gomock.Eq(TokenNameCameraEvent)).Return(nil, errors.New("err")).Times(2)

		b.handleRetryUploadEvents()
	})

	t.Run("upload camera event error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		server := S3StubServer()
		defer server.Close()

		dir, _ := ioutil.TempDir("", "images")
		cfg := configs.NewTestConfig(nil, configs.LogConfig{}, configs.RedisConfig{}, configs.BoxConfig{
			EventRetryCount: 5,
			EventSavedHours: 1,
			StoreDir:        dir,
		})
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)

		algoMap := &sync.Map{}
		algoMap.Store(EventTemperatureNormal, "normal")
		algoMap.Store(EventTemperatureAbnormal, "abnormal")
		algoMap.Store(EventQuestionnaireFail, "qrn_fail")

		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:       &cfg,
			configMux:    &sync.Mutex{},
			db:           data,
			apiClient:    client,
			logger:       zerolog.New(zerolog.NewConsoleWriter()),
			s3httpClient: &http.Client{},
			algoMap:      algoMap,
			camGroup:     camGroup,
			mux:          &sync.Mutex{},
			tokenCache:   &sync.Map{},
		}
		scan := structs.MetaScanData{
			Temperature: 30.0,
		}

		bt, _ := json.Marshal(scan)

		camGroup.EXPECT().GetCamera(gomock.Any()).Return(mock.NewMockThermal1Camera(ctrl), nil)
		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				ID:        4,
				PictureID: 1,
				Data:      strconv.Quote(string(bt)),
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
				CameraID:  1,
			},
		}, nil)

		data.EXPECT().GetFacePicture(gomock.Eq(uint(1))).Return(&db.FacePicture{
			Data: []byte(testPhoto),
		}, nil)

		data.EXPECT().IncrementRetryEvent(gomock.Eq(uint(4))).Return(nil)

		client.EXPECT().GetTokenByCamera(gomock.Any(), gomock.Eq(TokenNameCameraEvent)).Return(&cloud.Token{
			Url: server.URL,
		}, nil)
		client.EXPECT().UploadCameraEvent(gomock.Any()).Return(nil, errors.New("test_err"))

		b.handleRetryUploadEvents()
	})

	t.Run("successful upload", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		server := S3StubServer()
		defer server.Close()

		dir, _ := ioutil.TempDir("", "images")
		cfg := configs.NewTestConfig(nil, configs.LogConfig{}, configs.RedisConfig{}, configs.BoxConfig{
			EventRetryCount: 5,
			StoreDir:        dir,
			EventSavedHours: 1,
		})
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		algoMap := &sync.Map{}
		algoMap.Store(EventTemperatureNormal, "normal")
		algoMap.Store(EventTemperatureAbnormal, "abnormal")
		algoMap.Store(EventQuestionnaireFail, "qrn_fail")

		b := baseBox{
			config:       &cfg,
			configMux:    &sync.Mutex{},
			db:           data,
			apiClient:    client,
			logger:       zerolog.New(zerolog.NewConsoleWriter()),
			s3httpClient: &http.Client{},
			algoMap:      algoMap,
			camGroup:     camGroup,
			mux:          &sync.Mutex{},
			tokenCache:   &sync.Map{},
		}
		scan := structs.MetaScanData{
			Temperature: 30.0,
		}

		bt, _ := json.Marshal(scan)

		camGroup.EXPECT().GetCamera(gomock.Any()).Return(mock.NewMockThermal1Camera(ctrl), nil)
		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				ID:        5,
				PictureID: 1,
				Data:      strconv.Quote(string(bt)),
				CameraID:  3,
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)

		data.EXPECT().GetFacePicture(gomock.Eq(uint(1))).Return(&db.FacePicture{
			Data: []byte(testPhoto),
		}, nil)
		data.EXPECT().UpdateEvent(gomock.Eq(uint(5)), gomock.Eq("1"), gomock.Eq(int64(3))).Return(nil)

		client.EXPECT().GetTokenByCamera(gomock.Any(), gomock.Eq(TokenNameCameraEvent)).Return(&cloud.Token{
			Url: server.URL,
		}, nil)
		client.EXPECT().UploadCameraEvent(gomock.Any()).Return(&cloud.Event{
			ID: "1",
		}, nil)

		b.handleRetryUploadEvents()
	})

	t.Run("successful upload - delete", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		server := S3StubServer()
		defer server.Close()

		dir, _ := ioutil.TempDir("", "images")
		cfg := configs.NewTestConfig(nil, configs.LogConfig{}, configs.RedisConfig{}, configs.BoxConfig{
			EventRetryCount: 5,
			StoreDir:        dir,
			EventSavedHours: 1,
		})
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)

		algoMap := &sync.Map{}
		algoMap.Store(EventTemperatureNormal, "normal")
		algoMap.Store(EventTemperatureAbnormal, "abnormal")
		algoMap.Store(EventQuestionnaireFail, "qrn_fail")

		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:       &cfg,
			configMux:    &sync.Mutex{},
			db:           data,
			apiClient:    client,
			logger:       zerolog.New(zerolog.NewConsoleWriter()),
			s3httpClient: &http.Client{},
			algoMap:      algoMap,
			camGroup:     camGroup,
			mux:          &sync.Mutex{},
			tokenCache:   &sync.Map{},
		}
		scan := structs.MetaScanData{
			Temperature: 30.0,
		}

		bt, _ := json.Marshal(scan)

		camGroup.EXPECT().GetCamera(gomock.Any()).Return(mock.NewMockThermal1Camera(ctrl), nil)
		data.EXPECT().GetEventsWithoutCameraID().Return(nil, nil)
		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				ID:        5,
				PictureID: 1,
				Data:      strconv.Quote(string(bt)),
				CameraID:  3,
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)

		data.EXPECT().GetFacePicture(gomock.Eq(uint(1))).Return(&db.FacePicture{
			Data: []byte(testPhoto),
		}, nil)

		client.EXPECT().GetTokenByCamera(gomock.Any(), gomock.Eq(TokenNameCameraEvent)).Return(&cloud.Token{
			Url: server.URL,
		}, nil)
		client.EXPECT().UploadCameraEvent(gomock.Any()).Return(&cloud.Event{
			ID: "1",
		}, nil)
		data.EXPECT().UpdateEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		b.handleRetryUploadEvents()
	})

	t.Run("race condition - connect", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		server := S3StubServer()
		defer server.Close()

		dir, _ := ioutil.TempDir("", "images")
		cfg := configs.NewTestConfig(nil, configs.LogConfig{}, configs.RedisConfig{}, configs.BoxConfig{
			EventRetryCount: 5,
			StoreDir:        dir,
			EventSavedHours: 1,
		})
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)

		algoMap := &sync.Map{}
		algoMap.Store(EventTemperatureNormal, "normal")
		algoMap.Store(EventTemperatureAbnormal, "abnormal")
		algoMap.Store(EventQuestionnaireFail, "qrn_fail")

		camGroup := mock.NewMockCamGroup(ctrl)

		wg := sync.WaitGroup{}
		wg.Add(1)

		b := baseBox{
			config:       &cfg,
			configMux:    &sync.Mutex{},
			db:           data,
			apiClient:    client,
			logger:       zerolog.New(zerolog.NewConsoleWriter()),
			s3httpClient: &http.Client{},
			algoMap:      algoMap,
			boxInfo:      &configs.BoxInfo{},
			camGroup:     camGroup,
			mux:          &sync.Mutex{},
			tokenCache:   &sync.Map{},
		}
		scan := structs.MetaScanData{
			Temperature: 30.0,
		}

		bt, _ := json.Marshal(scan)

		camGroup.EXPECT().GetCamera(gomock.Any()).Return(mock.NewMockThermal1Camera(ctrl), nil)
		data.EXPECT().GetEventsWithoutCameraID().Return([]db.Event{
			db.Event{
				ID:        2,
				CameraSN:  "test_sn",
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)
		data.EXPECT().UpdateEvent(gomock.Eq(uint(2)), gomock.Eq(""), gomock.Eq(int64(1))).Return(nil)

		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				ID:        5,
				PictureID: 1,
				Data:      strconv.Quote(string(bt)),
				CameraID:  3,
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)

		data.EXPECT().GetFacePicture(gomock.Eq(uint(1))).Return(&db.FacePicture{
			Data: []byte(testPhoto),
		}, nil)
		data.EXPECT().UpdateEvent(gomock.Eq(uint(5)), gomock.Eq("1"), gomock.Eq(int64(3))).
			DoAndReturn(func(arg1, arg2, arg3 interface{}) error {
				wg.Done()
				return nil
			})

		client.EXPECT().GetCameras().Return([]*cloud.Camera{
			&cloud.Camera{
				SN: "test_sn",
				ID: 1,
			},
		}, nil)
		client.EXPECT().GetTokenByCamera(gomock.Any(), gomock.Eq(TokenNameCameraEvent)).Return(&cloud.Token{
			Url: server.URL,
		}, nil)
		client.EXPECT().UploadCameraEvent(gomock.Any()).Return(&cloud.Event{
			ID: "1",
		}, nil)

		client.EXPECT().Handshake().Return(errors.New("bad handshake")).AnyTimes()
		go b.handleRetryUploadEvents()

		for i := 0; i < 5; i++ {
			go b.ConnectCloud()
			go b.DisconnectCloud()
		}

		for i := 0; i < 5; i++ {
			b.ConnectCloud()
			b.DisconnectCloud()
		}

		assert.False(t, utils.WaitTimeout(&wg, time.Second))
	})

	t.Run("race condition - update", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		server := S3StubServer()
		defer server.Close()

		dir, _ := ioutil.TempDir("", "images")
		cfg := configs.NewTestConfig(nil, configs.LogConfig{}, configs.RedisConfig{}, configs.BoxConfig{
			EventRetryCount: 5,
			StoreDir:        dir,
			EventSavedHours: 1,
		})
		data := mock.NewMockDBClient(ctrl)
		client := mock.NewMockClient(ctrl)

		algoMap := &sync.Map{}
		algoMap.Store(EventTemperatureNormal, "normal")
		algoMap.Store(EventTemperatureAbnormal, "abnormal")
		algoMap.Store(EventQuestionnaireFail, "qrn_fail")

		camGroup := mock.NewMockCamGroup(ctrl)

		wg := sync.WaitGroup{}
		wg.Add(1)

		b := baseBox{
			config:       &cfg,
			configMux:    &sync.Mutex{},
			db:           data,
			apiClient:    client,
			logger:       zerolog.New(zerolog.NewConsoleWriter()),
			s3httpClient: &http.Client{},
			algoMap:      algoMap,
			boxInfo:      &configs.BoxInfo{},
			camGroup:     camGroup,
			mux:          &sync.Mutex{},
			tokenCache:   &sync.Map{},
		}
		scan := structs.MetaScanData{
			Temperature: 30.0,
		}

		bt, _ := json.Marshal(scan)

		camGroup.EXPECT().GetCamera(gomock.Any()).Return(mock.NewMockThermal1Camera(ctrl), nil)
		data.EXPECT().GetEventsWithoutCameraID().Return([]db.Event{
			db.Event{
				ID:        2,
				CameraSN:  "test_sn",
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)
		data.EXPECT().UpdateEvent(gomock.Eq(uint(2)), gomock.Eq(""), gomock.Eq(int64(1))).Return(nil)

		data.EXPECT().GetRetryEvents().Return([]db.Event{
			db.Event{
				ID:        5,
				PictureID: 1,
				Data:      strconv.Quote(string(bt)),
				CameraID:  3,
				Type:      EventTemperatureNormal,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)

		data.EXPECT().GetFacePicture(gomock.Eq(uint(1))).Return(&db.FacePicture{
			Data: []byte(testPhoto),
		}, nil)
		data.EXPECT().UpdateEvent(gomock.Eq(uint(5)), gomock.Eq("1"), gomock.Eq(int64(3))).
			DoAndReturn(func(arg1, arg2, arg3 interface{}) error {
				wg.Done()
				return nil
			})

		client.EXPECT().GetCameras().Return([]*cloud.Camera{
			&cloud.Camera{
				SN: "test_sn",
				ID: 1,
			},
		}, nil)
		client.EXPECT().GetTokenByCamera(gomock.Any(), gomock.Eq(TokenNameCameraEvent)).Return(&cloud.Token{
			Url: server.URL,
		}, nil)
		client.EXPECT().UploadCameraEvent(gomock.Any()).Return(&cloud.Event{
			ID: "1",
		}, nil)
		client.EXPECT().UpdateClientConfig(gomock.Any()).AnyTimes()

		go b.handleRetryUploadEvents()

		for i := 0; i < 5; i++ {
			go b.UpdateAPIClientConfig()
		}

		for i := 0; i < 5; i++ {
			b.UpdateAPIClientConfig()
		}

		assert.False(t, utils.WaitTimeout(&wg, time.Second))
	})
}

func TestHandleUploadEventVideos(t *testing.T) {
	t.Run("GetRetryEventVideos err", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
		}
		data.EXPECT().GetRetryEventVideos().Return(nil, errors.New("get event videos error"))
		b.handleRetryUploadEventVideos()
	})

	t.Run("no events", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
		}
		data.EXPECT().GetRetryEventVideos().Return([]db.Event{}, nil)
		b.handleRetryUploadEventVideos()
	})

	t.Run("time diff greater than event save hours plus 1", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
		}
		data.EXPECT().GetRetryEventVideos().Return([]db.Event{
			{
				ID:        1,
				CameraID:  1,
				PictureID: 1,
				CreatedAt: time.Now().Add(-2 * time.Hour),
			},
		}, nil)
		_ = cfg.SetEventSavedHours(1)
		data.EXPECT().UpdateEventVideo(uint(1), "", "", false).Return(nil)
		b.handleRetryUploadEventVideos()
	})

	t.Run("video retry count plus event retry count greater than retry limit in 1 hour", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
		}
		data.EXPECT().GetRetryEventVideos().Return([]db.Event{
			{
				ID:              1,
				CameraID:        1,
				PictureID:       1,
				CreatedAt:       time.Now().Add(-6 * time.Minute),
				RetryCount:      0,
				VideoRetryCount: 6,
			},
		}, nil)
		_ = cfg.SetEventSavedHours(1)
		_ = cfg.SetEventRetryCount(5)
		b.handleRetryUploadEventVideos()
	})

	t.Run("video retry count plus event retry count greater than retry limit in 2 hour", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
		}
		data.EXPECT().GetRetryEventVideos().Return([]db.Event{
			{
				ID:              1,
				CameraID:        1,
				PictureID:       1,
				CreatedAt:       time.Now().Add(-1 * time.Hour),
				RetryCount:      3,
				VideoRetryCount: 8,
			},
		}, nil)
		_ = cfg.SetEventSavedHours(3)
		_ = cfg.SetEventRetryCount(5)
		b.handleRetryUploadEventVideos()
	})

	t.Run("get camera err", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			camGroup:  camGroup,
		}
		data.EXPECT().GetRetryEventVideos().Return([]db.Event{
			{
				ID:        1,
				CameraID:  1,
				PictureID: 1,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)
		_ = cfg.SetEventSavedHours(1)
		_ = cfg.SetEventRetryCount(5)
		camGroup.EXPECT().GetCamera(gomock.Any()).Return(nil, errors.New("err get camera"))
		b.handleRetryUploadEventVideos()
	})

	t.Run("aicamera err", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			camGroup:  camGroup,
		}
		data.EXPECT().GetRetryEventVideos().Return([]db.Event{
			{
				ID:        1,
				CameraID:  1,
				PictureID: 1,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)
		_ = cfg.SetEventSavedHours(1)
		_ = cfg.SetEventRetryCount(5)
		camGroup.EXPECT().GetCamera(gomock.Any()).Return(mock.NewMockThermal1Camera(ctrl), nil)
		b.handleRetryUploadEventVideos()
	})

	t.Run("aicamera ok", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		data := mock.NewMockDBClient(ctrl)
		camGroup := mock.NewMockCamGroup(ctrl)

		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			db:        data,
			camGroup:  camGroup,
		}
		data.EXPECT().GetRetryEventVideos().Return([]db.Event{
			{
				ID:        1,
				CameraID:  1,
				PictureID: 1,
				CreatedAt: time.Now().Add(-6 * time.Minute),
			},
		}, nil)
		_ = cfg.SetEventSavedHours(1)
		_ = cfg.SetEventRetryCount(5)
		aiCamera := mock.NewMockAICamera(ctrl)
		camGroup.EXPECT().GetCamera(gomock.Any()).Return(aiCamera, nil)
		aiCamera.EXPECT().GetManufacturer().Return(utils.TuringUniview)
		aiCamera.EXPECT().RecordVideo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", 0, 0, errors.New("record video error"))
		data.EXPECT().IncrementRetryEventVideo(uint(1)).Return(nil)
		data.EXPECT().UpdateEventVideo(uint(1), "", "", true).Return(nil)
		b.handleRetryUploadEventVideos()
	})
}

func TestCloudConnection(t *testing.T) {
	stopCh := make(chan struct{})
	clearCh := make(chan struct{})
	defer func() {
		close(stopCh)
		close(clearCh)
	}()
	address := "127.0.0.1:10002"
	// start test ws server
	outCh, _ := startWSServer(address, stopCh, clearCh, t)

	// wait ws server started
	time.Sleep(100 * time.Millisecond)

	t.Run("test ws reconnect successful with cookie", func(t *testing.T) {

		// mock client init
		mockCtl := gomock.NewController(t)
		mockConfig := mock.NewMockConfig(mockCtl)
		mockHttpClient := mock.NewMockClient(mockCtl)
		b := baseBox{
			config:    mockConfig,
			configMux: &sync.Mutex{},
			apiClient: mockHttpClient,
		}
		mockConfig.EXPECT().GetLevel()
		mockConfig.EXPECT().GetSoftwareVersion()
		mockConfig.EXPECT().GetWebsocketPingPeriod().Return(100 * time.Millisecond)
		mockConfig.EXPECT().GetWebsocketReconnectSleepPeriod().Return(100 * time.Millisecond)
		mockConfig.EXPECT().GetWebsocketServerUrl().Return(fmt.Sprintf("ws://%s", address))

		var testCookie = &http.Cookie{
			Name:  "test-cookie",
			Value: "test-value",
		}

		mockHttpClient.EXPECT().Cookies().Do(func() {
			t.Log("return Cookies: ", testCookie.String())
		}).Return([]*http.Cookie{testCookie}).AnyTimes()
		handshakeCount := 0
		mockHttpClient.EXPECT().Handshake().Do(func() {
			handshakeCount++
		}).AnyTimes()

		// start to test ws-server connection
		err := b.connectWebsocketServer()
		if assert.NoError(t, err) {
			reply := <-outCh
			t.Log("assert ws reply")
			assert.Equal(t, "cookie", reply.t)
			assert.Equal(t, testCookie.String(), string(reply.msg))
			assert.Equal(t, 1, handshakeCount)
		}
		clearCh <- struct{}{}

		time.Sleep(100 * time.Millisecond)
		b.wsClient.Send([]byte("closed conn"))

		// wait ws connection reconnect
		time.Sleep(2 * time.Second)
		assert.Equal(t, 2, handshakeCount)
		return
	})
}

type signal struct {
	// type of message
	t string // enum{"stopped", "data"}

	// detailed message
	msg []byte
}

func startWSServer(addr string, stopCh <-chan struct{}, clearConn <-chan struct{}, t *testing.T) (<-chan signal, chan<- signal) {
	output := make(chan signal, 1)
	input := make(chan signal, 1)

	go func(stopCh <-chan struct{}) {

		connList := make([]*websocket.Conn, 0)

		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}
		h := http.NewServeMux()
		h.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			c, err := upgrader.Upgrade(writer, request, nil)
			if err != nil {
				panic(err)
			}
			defer c.Close()

			connList = append(connList, c)
			output <- signal{"cookie", []byte(request.Header.Get("Cookie"))}
			for {
				_, message, err := c.ReadMessage()
				if err != nil {
					break
				}
				// NOTE: here assumes a paired req / res message pattern
				output <- signal{"data", message}
				reply := <-input
				if err := c.WriteMessage(websocket.TextMessage, reply.msg); err != nil {
					panic(err)
				}
			}
		})
		s := http.Server{
			Addr:    addr,
			Handler: h,
		}
		var errCh = make(chan error, 1)
		go func() {
			errCh <- s.ListenAndServe()
		}()

		for {
			select {
			case <-stopCh:
				s.Close()
				output <- signal{"stopped", nil}
				return
			case <-errCh:
				return
			case <-clearConn:
				for _, c := range connList {
					c.Close()
				}
			}
		}
	}(stopCh)
	return output, input
}

func TestUpdateSnapshotRule(t *testing.T) {
	t.Run("no license", func(t *testing.T) {
		// mock client init
		cloud.ClearCameraSettings()
		cloud.SaveCameraSettings([]*cloud.CameraSettings{
			{
				CamID:           1,
				CamSN:           "sn1",
				CloudEventTypes: []string{},
			},
		})
		mockCtl := gomock.NewController(t)
		aiCam := mock.NewMockAICamera(mockCtl)
		aiCam.EXPECT().GetID().Return(1)
		nvrManager := mock.NewMockNVRManager(mockCtl)

		camGroup := mock.NewMockCamGroup(mockCtl)
		camGroup.EXPECT().AllCameras().Return([]base.Camera{aiCam})
		b := baseBox{
			configMux:  &sync.Mutex{},
			camGroup:   camGroup,
			nvrManager: nvrManager,
		}
		b.updateSnapshotRule()
	})
	t.Run("1 camera with license", func(t *testing.T) {
		// mock client init
		cloud.ClearCameraSettings()
		cloud.SaveCameraSettings([]*cloud.CameraSettings{
			{
				CamID:           1,
				CamSN:           "sn1",
				CloudEventTypes: []string{cloud.MotionStart},
			},
		})
		mockCtl := gomock.NewController(t)
		aiCam := mock.NewMockAICamera(mockCtl)
		aiCam.EXPECT().GetID().Return(1)
		aiCam.EXPECT().GetNvrSN().Return("nvr1")

		nvrCli := uniview.NewMockClient(mockCtl)
		nvrCli.EXPECT().PutSystemSnapshotRule(gomock.Eq(true)).Return(nil).Times(1)
		nvrManager := mock.NewMockNVRManager(mockCtl)
		nvrManager.EXPECT().GetNVRClientBySN(gomock.Any()).Return(nvrCli, nil).Times(1)

		camGroup := mock.NewMockCamGroup(mockCtl)
		camGroup.EXPECT().AllCameras().Return([]base.Camera{aiCam})
		b := baseBox{
			configMux:  &sync.Mutex{},
			camGroup:   camGroup,
			nvrManager: nvrManager,
		}
		b.updateSnapshotRule()
	})
	t.Run("no license on last camera", func(t *testing.T) {
		// mock client init
		cloud.ClearCameraSettings()
		cloud.SaveCameraSettings([]*cloud.CameraSettings{
			{
				CamID:           1,
				CamSN:           "sn1",
				CloudEventTypes: []string{cloud.MotionStart},
			},
			{
				CamID:           2,
				CamSN:           "sn2",
				CloudEventTypes: []string{},
			},
		})
		mockCtl := gomock.NewController(t)
		aiCam1 := mock.NewMockAICamera(mockCtl)
		aiCam1.EXPECT().GetID().Return(1)
		aiCam1.EXPECT().GetNvrSN().Return("nvr1").Times(1)
		aiCam2 := mock.NewMockAICamera(mockCtl)
		aiCam2.EXPECT().GetID().Return(2)

		nvrCli := uniview.NewMockClient(mockCtl)
		nvrCli.EXPECT().PutSystemSnapshotRule(gomock.Eq(true)).Return(nil).Times(1)
		nvrManager := mock.NewMockNVRManager(mockCtl)
		nvrManager.EXPECT().GetNVRClientBySN(gomock.Any()).Return(nvrCli, nil).Times(1)

		camGroup := mock.NewMockCamGroup(mockCtl)
		camGroup.EXPECT().AllCameras().Return([]base.Camera{aiCam1, aiCam2})
		b := baseBox{
			configMux:  &sync.Mutex{},
			camGroup:   camGroup,
			nvrManager: nvrManager,
		}
		b.updateSnapshotRule()
	})
	t.Run("no license on first camera", func(t *testing.T) {
		// mock client init
		cloud.ClearCameraSettings()
		cloud.SaveCameraSettings([]*cloud.CameraSettings{
			{
				CamID:           1,
				CamSN:           "sn1",
				CloudEventTypes: []string{},
			},
			{
				CamID:           2,
				CamSN:           "sn2",
				CloudEventTypes: []string{cloud.MotionStart},
			},
		})
		mockCtl := gomock.NewController(t)
		aiCam1 := mock.NewMockAICamera(mockCtl)
		aiCam1.EXPECT().GetID().Return(1)
		aiCam2 := mock.NewMockAICamera(mockCtl)
		aiCam2.EXPECT().GetID().Return(2)
		aiCam2.EXPECT().GetNvrSN().Return("nvr1").Times(1)

		nvrCli := uniview.NewMockClient(mockCtl)
		nvrCli.EXPECT().PutSystemSnapshotRule(gomock.Eq(true)).Return(nil).Times(1)
		nvrManager := mock.NewMockNVRManager(mockCtl)
		nvrManager.EXPECT().GetNVRClientBySN(gomock.Any()).Return(nvrCli, nil).Times(1)

		camGroup := mock.NewMockCamGroup(mockCtl)
		camGroup.EXPECT().AllCameras().Return([]base.Camera{aiCam1, aiCam2})
		b := baseBox{
			configMux:  &sync.Mutex{},
			camGroup:   camGroup,
			nvrManager: nvrManager,
		}
		b.updateSnapshotRule()
	})
	t.Run("license on all cameras", func(t *testing.T) {
		// mock client init
		cloud.ClearCameraSettings()
		cloud.SaveCameraSettings([]*cloud.CameraSettings{
			{
				CamID:           1,
				CamSN:           "sn1",
				CloudEventTypes: []string{cloud.MotionStart},
			},
			{
				CamID:           2,
				CamSN:           "sn2",
				CloudEventTypes: []string{cloud.MotionStart},
			},
		})
		mockCtl := gomock.NewController(t)
		aiCam1 := mock.NewMockAICamera(mockCtl)
		aiCam1.EXPECT().GetID().Return(1)
		aiCam1.EXPECT().GetNvrSN().Return("nvr1").Times(1)
		aiCam2 := mock.NewMockAICamera(mockCtl)
		aiCam2.EXPECT().GetID().Return(2)
		aiCam2.EXPECT().GetNvrSN().Return("nvr1").Times(1)

		nvrCli := uniview.NewMockClient(mockCtl)
		nvrCli.EXPECT().PutSystemSnapshotRule(gomock.Eq(true)).Return(nil).Times(1)
		nvrManager := mock.NewMockNVRManager(mockCtl)
		nvrManager.EXPECT().GetNVRClientBySN(gomock.Any()).Return(nvrCli, nil).Times(1)

		camGroup := mock.NewMockCamGroup(mockCtl)
		camGroup.EXPECT().AllCameras().Return([]base.Camera{aiCam1, aiCam2})
		b := baseBox{
			configMux:  &sync.Mutex{},
			camGroup:   camGroup,
			nvrManager: nvrManager,
		}
		b.updateSnapshotRule()
	})
	t.Run("camera plugin in different nvr", func(t *testing.T) {
		// mock client init
		cloud.ClearCameraSettings()
		cloud.SaveCameraSettings([]*cloud.CameraSettings{
			{
				CamID:           1,
				CamSN:           "sn1",
				CloudEventTypes: []string{cloud.MotionStart},
			},
			{
				CamID:           2,
				CamSN:           "sn2",
				CloudEventTypes: []string{cloud.MotionStart},
			},
		})
		mockCtl := gomock.NewController(t)
		aiCam1 := mock.NewMockAICamera(mockCtl)
		aiCam1.EXPECT().GetID().Return(1)
		aiCam1.EXPECT().GetNvrSN().Return("nvr1").Times(1)
		aiCam2 := mock.NewMockAICamera(mockCtl)
		aiCam2.EXPECT().GetID().Return(2)
		aiCam2.EXPECT().GetNvrSN().Return("nvr2").Times(1)

		nvrCli1 := uniview.NewMockClient(mockCtl)
		nvrCli1.EXPECT().PutSystemSnapshotRule(gomock.Eq(true)).Return(nil).Times(1)
		nvrCli2 := uniview.NewMockClient(mockCtl)
		nvrCli2.EXPECT().PutSystemSnapshotRule(gomock.Eq(true)).Return(nil).Times(1)
		nvrManager := mock.NewMockNVRManager(mockCtl)
		nvrManager.EXPECT().GetNVRClientBySN(gomock.Eq("nvr1")).Return(nvrCli1, nil).Times(1)
		nvrManager.EXPECT().GetNVRClientBySN(gomock.Eq("nvr2")).Return(nvrCli2, nil).Times(1)

		camGroup := mock.NewMockCamGroup(mockCtl)
		camGroup.EXPECT().AllCameras().Return([]base.Camera{aiCam1, aiCam2})
		b := baseBox{
			configMux:  &sync.Mutex{},
			camGroup:   camGroup,
			nvrManager: nvrManager,
		}
		b.updateSnapshotRule()
	})
}

func Test_isSameVideoStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		src  []uniview.VideoStreamInfo
		dest []uniview.VideoStreamInfo
		eq   bool
	}{
		{
			src:  []uniview.VideoStreamInfo{},
			dest: []uniview.VideoStreamInfo{},
			eq:   true,
		},
		{
			src: []uniview.VideoStreamInfo{{
				ID:              1,
				MainStreamType:  1,
				Enabled:         1,
				VideoEncodeInfo: uniview.VideoEncodeInfo{},
			}},
			dest: []uniview.VideoStreamInfo{},
			eq:   false,
		},
		{
			src: []uniview.VideoStreamInfo{
				{
					ID:              1,
					MainStreamType:  1,
					Enabled:         1,
					VideoEncodeInfo: uniview.VideoEncodeInfo{},
				},
			},
			dest: []uniview.VideoStreamInfo{
				{
					ID:              1,
					MainStreamType:  1,
					Enabled:         1,
					VideoEncodeInfo: uniview.VideoEncodeInfo{},
				},
			},
			eq: true,
		},
		{
			src: []uniview.VideoStreamInfo{
				{
					ID:              1,
					MainStreamType:  1,
					Enabled:         1,
					VideoEncodeInfo: uniview.VideoEncodeInfo{},
				},
				{
					ID:              2,
					MainStreamType:  1,
					Enabled:         1,
					VideoEncodeInfo: uniview.VideoEncodeInfo{},
				},
			},
			dest: []uniview.VideoStreamInfo{
				{
					ID:              1,
					MainStreamType:  1,
					Enabled:         1,
					VideoEncodeInfo: uniview.VideoEncodeInfo{},
				},
			},
			eq: false,
		},
		{
			src: []uniview.VideoStreamInfo{
				{
					ID:             1,
					MainStreamType: 1,
					Enabled:        1,
					VideoEncodeInfo: uniview.VideoEncodeInfo{
						BitRate: 100,
					},
				},
			},
			dest: []uniview.VideoStreamInfo{
				{
					ID:             1,
					MainStreamType: 1,
					Enabled:        1,
					VideoEncodeInfo: uniview.VideoEncodeInfo{
						BitRate: 101,
					},
				},
			},
			eq: false,
		},
		{
			src: []uniview.VideoStreamInfo{
				{
					ID:             1,
					MainStreamType: 1,
					Enabled:        1,
					VideoEncodeInfo: uniview.VideoEncodeInfo{
						BitRate: 100,
					},
				},
				{
					ID:             2,
					MainStreamType: 1,
					Enabled:        1,
					VideoEncodeInfo: uniview.VideoEncodeInfo{
						BitRate: 101,
					},
				},
			},
			dest: []uniview.VideoStreamInfo{
				{
					ID:             2,
					MainStreamType: 1,
					Enabled:        1,
					VideoEncodeInfo: uniview.VideoEncodeInfo{
						BitRate: 101,
					},
				},
				{
					ID:             1,
					MainStreamType: 1,
					Enabled:        1,
					VideoEncodeInfo: uniview.VideoEncodeInfo{
						BitRate: 100,
					},
				},
			},
			eq: true,
		},
	}
	for _, test := range tests {
		t.Run("isSameVideoStream", func(t *testing.T) {
			assert.Equal(t, test.eq, isSameVideoStream(test.src, test.dest))
		})
	}
}

func Test_isSameAudioSettings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tests := []struct {
		src  []uniview.AudioStatus
		dest []uniview.AudioStatus
		eq   bool
	}{
		{
			src:  []uniview.AudioStatus{},
			dest: []uniview.AudioStatus{},
			eq:   true,
		},
		{
			src: []uniview.AudioStatus{
				{
					StreamID:      0,
					IsDecodeAudio: 1,
				},
			},
			dest: []uniview.AudioStatus{},
			eq:   false,
		},
		{
			src: []uniview.AudioStatus{
				{
					StreamID:      0,
					IsDecodeAudio: 1,
				},
			},
			dest: []uniview.AudioStatus{
				{
					StreamID:      0,
					IsDecodeAudio: 1,
				},
			},
			eq: true,
		},
		{
			src: []uniview.AudioStatus{
				{
					StreamID:      0,
					IsDecodeAudio: 1,
				},
				{
					StreamID:      1,
					IsDecodeAudio: 1,
				},
			},
			dest: []uniview.AudioStatus{
				{
					StreamID:      0,
					IsDecodeAudio: 1,
				},
			},
			eq: false,
		},
		{
			src: []uniview.AudioStatus{
				{
					StreamID:      0,
					IsDecodeAudio: 1,
				},
				{
					StreamID:      1,
					IsDecodeAudio: 1,
				},
			},
			dest: []uniview.AudioStatus{
				{
					StreamID:      0,
					IsDecodeAudio: 1,
				},
				{
					StreamID:      1,
					IsDecodeAudio: 0,
				},
			},
			eq: false,
		},
		{
			src: []uniview.AudioStatus{
				{
					StreamID:      0,
					IsDecodeAudio: 1,
				},
				{
					StreamID:      1,
					IsDecodeAudio: 1,
				},
			},
			dest: []uniview.AudioStatus{
				{
					StreamID:      0,
					IsDecodeAudio: 1,
				},
				{
					StreamID:      1,
					IsDecodeAudio: 1,
				},
			},
			eq: true,
		},
		{
			src: []uniview.AudioStatus{
				{
					StreamID:      0,
					IsDecodeAudio: 1,
				},
				{
					StreamID:      1,
					IsDecodeAudio: 1,
				},
			},
			dest: []uniview.AudioStatus{
				{
					StreamID:      1,
					IsDecodeAudio: 1,
				},
				{
					StreamID:      0,
					IsDecodeAudio: 1,
				},
			},
			eq: true,
		},
	}
	for _, test := range tests {
		t.Run("isSameAudioSettings", func(t *testing.T) {
			assert.Equal(t, test.eq, isSameAudioSettings(test.src, test.dest))
		})
	}
}

func Test_isSameMediaVideoCapabilities(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tests := []struct {
		src  *uniview.MediaVideoCapabilities
		dest *uniview.MediaVideoCapabilities
		eq   bool
	}{
		{
			src:  nil,
			dest: nil,
			eq:   true,
		},
		{
			src:  &uniview.MediaVideoCapabilities{},
			dest: nil,
			eq:   false,
		},
		{
			src:  nil,
			dest: &uniview.MediaVideoCapabilities{},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{},
			dest: &uniview.MediaVideoCapabilities{},
			eq:   true,
		},
		{
			src:  &uniview.MediaVideoCapabilities{IsSupportCfg: 1},
			dest: &uniview.MediaVideoCapabilities{IsSupportCfg: 0},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{IsSupportSmoothLevel: 1},
			dest: &uniview.MediaVideoCapabilities{IsSupportSmoothLevel: 0},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{IsSupportImageFormat: 1},
			dest: &uniview.MediaVideoCapabilities{IsSupportImageFormat: 0},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{IsSupportScrambled: 1},
			dest: &uniview.MediaVideoCapabilities{IsSupportScrambled: 0},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{EncodeFormatNum: 1},
			dest: &uniview.MediaVideoCapabilities{EncodeFormatNum: 0},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{EncodeFormatNum: 1},
			dest: &uniview.MediaVideoCapabilities{EncodeFormatNum: 0},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{MinIFrameInterval: 1},
			dest: &uniview.MediaVideoCapabilities{MinIFrameInterval: 0},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{MaxIFrameInterval: 1},
			dest: &uniview.MediaVideoCapabilities{MaxIFrameInterval: 0},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{StreamCapabilityNum: 1},
			dest: &uniview.MediaVideoCapabilities{StreamCapabilityNum: 0},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{VideoModeNum: 1},
			dest: &uniview.MediaVideoCapabilities{VideoModeNum: 0},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{GOPTypeNum: 1},
			dest: &uniview.MediaVideoCapabilities{GOPTypeNum: 0},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{EncodeFormatList: []int{}},
			dest: &uniview.MediaVideoCapabilities{EncodeFormatList: []int{}},
			eq:   true,
		},
		{
			src:  &uniview.MediaVideoCapabilities{EncodeFormatList: []int{1}},
			dest: &uniview.MediaVideoCapabilities{EncodeFormatList: []int{}},
			eq:   false,
		},
		{
			src:  &uniview.MediaVideoCapabilities{EncodeFormatList: []int{1, 2}},
			dest: &uniview.MediaVideoCapabilities{EncodeFormatList: []int{2, 1}},
			eq:   true,
		},
		{
			src: &uniview.MediaVideoCapabilities{
				VideoModeInfoList: []uniview.VideoModeInfo{
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 100,
					},
				},
			},
			dest: &uniview.MediaVideoCapabilities{
				VideoModeInfoList: []uniview.VideoModeInfo{
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 100,
					},
				},
			},
			eq: true,
		},
		{
			src: &uniview.MediaVideoCapabilities{
				VideoModeInfoList: []uniview.VideoModeInfo{
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 100,
					},
				},
			},
			dest: &uniview.MediaVideoCapabilities{
				VideoModeInfoList: []uniview.VideoModeInfo{
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 100,
					},
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 101,
					},
				},
			},
			eq: false,
		},
		{
			src: &uniview.MediaVideoCapabilities{
				VideoModeInfoList: []uniview.VideoModeInfo{
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 101,
					},
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 100,
					},
				},
			},
			dest: &uniview.MediaVideoCapabilities{
				VideoModeInfoList: []uniview.VideoModeInfo{
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 100,
					},
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 101,
					},
				},
			},
			eq: true,
		},
		{
			src: &uniview.MediaVideoCapabilities{
				VideoModeInfoList: []uniview.VideoModeInfo{
					{
						Resolution: uniview.Resolution{
							Width:  1900,
							Height: 1080,
						},
						FrameRate: 101,
					},
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 100,
					},
				},
			},
			dest: &uniview.MediaVideoCapabilities{
				VideoModeInfoList: []uniview.VideoModeInfo{
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 100,
					},
					{
						Resolution: uniview.Resolution{
							Width:  1920,
							Height: 1080,
						},
						FrameRate: 101,
					},
				},
			},
			eq: false,
		},
		{
			src: &uniview.MediaVideoCapabilities{
				StreamCapabilityList: []uniview.StreamCapability{},
			},
			dest: &uniview.MediaVideoCapabilities{
				StreamCapabilityList: []uniview.StreamCapability{},
			},
			eq: true,
		},
		{
			src: &uniview.MediaVideoCapabilities{
				StreamCapabilityList: []uniview.StreamCapability{
					{ID: 1},
				},
			},
			dest: &uniview.MediaVideoCapabilities{
				StreamCapabilityList: []uniview.StreamCapability{
					{ID: 2},
				},
			},
			eq: false,
		},
		{
			src: &uniview.MediaVideoCapabilities{
				StreamCapabilityList: []uniview.StreamCapability{
					{ID: 1},
					{ID: 2},
				},
			},
			dest: &uniview.MediaVideoCapabilities{
				StreamCapabilityList: []uniview.StreamCapability{
					{ID: 2},
				},
			},
			eq: false,
		},
		{
			src: &uniview.MediaVideoCapabilities{
				StreamCapabilityList: []uniview.StreamCapability{
					{ID: 1},
					{ID: 2},
				},
			},
			dest: &uniview.MediaVideoCapabilities{
				StreamCapabilityList: []uniview.StreamCapability{
					{ID: 2},
					{ID: 1},
				},
			},
			eq: true,
		},
	}
	for _, test := range tests {
		t.Run("isSameMediaVideoCapabilities", func(t *testing.T) {
			assert.Equal(t, test.eq, isSameMediaVideoCapabilities(test.src, test.dest))
		})
	}
}

func Test_isSameOSDSettings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tests := []struct {
		src  *uniview.OSDSetting
		dest *uniview.OSDSetting
		eq   bool
	}{
		{
			src:  nil,
			dest: nil,
			eq:   true,
		},
		{
			src:  &uniview.OSDSetting{},
			dest: nil,
			eq:   false,
		},
		{
			src:  nil,
			dest: &uniview.OSDSetting{},
			eq:   false,
		},
		{
			src:  &uniview.OSDSetting{Num: 1},
			dest: &uniview.OSDSetting{Num: 2},
			eq:   false,
		},
		{
			src:  &uniview.OSDSetting{ContentStyle: uniview.OSDContentStyle{FontSize: 1}},
			dest: &uniview.OSDSetting{ContentStyle: uniview.OSDContentStyle{FontSize: 2}},
			eq:   false,
		},
		{
			src:  &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1}}},
			dest: &uniview.OSDSetting{ContentList: []uniview.OSDContent{}},
			eq:   false,
		},
		{
			src:  &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, Enabled: 1}}},
			dest: &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, Enabled: 0}}},
			eq:   false,
		},
		{
			src:  &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, Num: 1}}},
			dest: &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, Num: 0}}},
			eq:   false,
		},
		{
			src:  &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, Area: uniview.OSDArea{uniview.OSDTopLeft{X: 1, Y: 2}}}}},
			dest: &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, Area: uniview.OSDArea{uniview.OSDTopLeft{X: 2, Y: 1}}}}},
			eq:   false,
		},
		{
			src:  &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, ContentInfoList: []uniview.OSDContentInfo{}}}},
			dest: &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, ContentInfoList: []uniview.OSDContentInfo{{ContentType: 1}}}}},
			eq:   false,
		},
		{
			src:  &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, ContentInfoList: []uniview.OSDContentInfo{{ContentType: 2}}}}},
			dest: &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, ContentInfoList: []uniview.OSDContentInfo{{ContentType: 1}}}}},
			eq:   false,
		},
		{
			src:  &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, ContentInfoList: []uniview.OSDContentInfo{{ContentType: 2}}}}},
			dest: &uniview.OSDSetting{ContentList: []uniview.OSDContent{{ID: 1, ContentInfoList: []uniview.OSDContentInfo{{ContentType: 2}}}}},
			eq:   true,
		},
	}
	for _, test := range tests {
		t.Run("isSameOSDSettings", func(t *testing.T) {
			assert.Equal(t, test.eq, isSameOSDSettings(test.src, test.dest))
		})
	}
}

func Test_isOSDCapabilities(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tests := []struct {
		src  *uniview.OSDCapabilities
		dest *uniview.OSDCapabilities
		eq   bool
	}{
		{
			src:  nil,
			dest: nil,
			eq:   true,
		},
		{
			src:  &uniview.OSDCapabilities{},
			dest: nil,
			eq:   false,
		},
		{
			src:  nil,
			dest: &uniview.OSDCapabilities{},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{},
			dest: &uniview.OSDCapabilities{},
			eq:   true,
		},
		{
			src:  &uniview.OSDCapabilities{IsSupportCfg: 1},
			dest: &uniview.OSDCapabilities{IsSupportCfg: 0},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{SupportedOSDContentTypeNum: 1},
			dest: &uniview.OSDCapabilities{SupportedOSDContentTypeNum: 0},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{IsSupportFontSizeCfg: 1},
			dest: &uniview.OSDCapabilities{IsSupportFontSizeCfg: 0},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{IsSupportFontColorCfg: 1},
			dest: &uniview.OSDCapabilities{IsSupportFontColorCfg: 0},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{MaxAreaNum: 1},
			dest: &uniview.OSDCapabilities{MaxAreaNum: 0},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{MaxOSDNum: 1},
			dest: &uniview.OSDCapabilities{MaxOSDNum: 0},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{MaxPerAreaOSDNum: 1},
			dest: &uniview.OSDCapabilities{MaxPerAreaOSDNum: 0},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{SupportedTimeFormatNum: 1},
			dest: &uniview.OSDCapabilities{SupportedTimeFormatNum: 0},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{SupportedDateFormatNum: 1},
			dest: &uniview.OSDCapabilities{SupportedDateFormatNum: 0},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{SupportedOSDContentTypeList: []int{1}},
			dest: &uniview.OSDCapabilities{SupportedOSDContentTypeList: []int{}},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{SupportedOSDContentTypeList: []int{1, 2}},
			dest: &uniview.OSDCapabilities{SupportedOSDContentTypeList: []int{2, 1}},
			eq:   true,
		},
		{
			src:  &uniview.OSDCapabilities{SupportedTimeFormatList: []int{1}},
			dest: &uniview.OSDCapabilities{SupportedTimeFormatList: []int{}},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{SupportedTimeFormatList: []int{1, 2}},
			dest: &uniview.OSDCapabilities{SupportedTimeFormatList: []int{2, 1}},
			eq:   true,
		},
		{
			src:  &uniview.OSDCapabilities{SupportedDateFormatList: []int{1}},
			dest: &uniview.OSDCapabilities{SupportedDateFormatList: []int{}},
			eq:   false,
		},
		{
			src:  &uniview.OSDCapabilities{SupportedDateFormatList: []int{1, 2}},
			dest: &uniview.OSDCapabilities{SupportedDateFormatList: []int{2, 1}},
			eq:   true,
		},
	}
	for _, test := range tests {
		t.Run("isSameOSDCapabilities", func(t *testing.T) {
			assert.Equal(t, test.eq, isSameOSDCapabilities(test.src, test.dest))
		})
	}
}

func Test_isCameraSettingsChanged(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tests := []struct {
		local  *cloud.CameraSettings
		remote *cloud.CameraSettings
		eq     bool
	}{
		{
			local:  nil,
			remote: &cloud.CameraSettings{},
			eq:     false,
		},
		{
			local:  &cloud.CameraSettings{},
			remote: nil,
			eq:     true,
		},
		{
			local: &cloud.CameraSettings{
				StreamSettings: []uniview.VideoStreamInfo{
					{ID: 1},
				},
			},
			remote: &cloud.CameraSettings{
				StreamSettings: []uniview.VideoStreamInfo{
					{ID: 2},
				},
			},
			eq: true,
		},
		{
			local: &cloud.CameraSettings{
				AudioSettings: []uniview.AudioStatus{
					{StreamID: 1},
				},
			},
			remote: &cloud.CameraSettings{
				AudioSettings: []uniview.AudioStatus{
					{StreamID: 2},
				},
			},
			eq: true,
		},
		{
			local: &cloud.CameraSettings{
				VideoCapabilities: &uniview.MediaVideoCapabilities{
					IsSupportCfg: 1,
				},
			},
			remote: &cloud.CameraSettings{
				VideoCapabilities: &uniview.MediaVideoCapabilities{
					IsSupportCfg: 0,
				},
			},
			eq: true,
		},
		{
			local: &cloud.CameraSettings{
				OSDSettings: &uniview.OSDSetting{
					Num: 1,
				},
			},
			remote: &cloud.CameraSettings{
				OSDSettings: &uniview.OSDSetting{
					Num: 2,
				},
			},
			eq: true,
		},
		{
			local: &cloud.CameraSettings{
				OSDCapabilities: &uniview.OSDCapabilities{
					IsSupportCfg: 1,
				},
			},
			remote: &cloud.CameraSettings{
				OSDCapabilities: &uniview.OSDCapabilities{
					IsSupportCfg: 0,
				},
			},
			eq: true,
		},
		{
			local:  &cloud.CameraSettings{},
			remote: &cloud.CameraSettings{},
			eq:     false,
		},
	}
	for _, test := range tests {
		t.Run("isCameraSettingsChanged", func(t *testing.T) {
			assert.Equal(t, test.eq, isCameraSettingsChanged(test.local, test.remote))
		})
	}
}

func Test_fetchCameraSettingsFromNvr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	var nvrMock *mock.MockNVRManager
	var cam *mock.MockAICamera
	var box *baseBox
	var cfg *mock.MockConfig

	init := func() {
		nvrMock = mock.NewMockNVRManager(ctrl)
		cam = mock.NewMockAICamera(ctrl)
		cfg = mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		box = New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		box.nvrManager = nvrMock
		cloud.ClearCameraSettings()
	}
	t.Run("nvrCli not found", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nil, errors.New("nvr cli not found"))
		ret := box.fetchCameraSettingsFromNvr(cam)
		assert.Nil(t, ret)
	})
	t.Run("get stream info err", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(nil, errors.New("err"))
		ret := box.fetchCameraSettingsFromNvr(cam)
		assert.Nil(t, ret)
	})
	t.Run("get video capabilities err", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(&uniview.VideoStreamInfos{
			Num:              0,
			VideoStreamInfos: []uniview.VideoStreamInfo{},
		}, nil)
		nvrCli.EXPECT().GetChannelMediaVideoCapabilities(uint32(1)).Return(nil, errors.New("err"))
		ret := box.fetchCameraSettingsFromNvr(cam)
		assert.Nil(t, ret)
	})
	t.Run("get audio switch info err", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(&uniview.VideoStreamInfos{
			Num:              0,
			VideoStreamInfos: []uniview.VideoStreamInfo{},
		}, nil)
		nvrCli.EXPECT().GetChannelMediaVideoCapabilities(uint32(1)).Return(&uniview.MediaVideoCapabilities{}, nil)
		nvrCli.EXPECT().GetAudioDecodeStatuses(uint32(1)).Return(nil, errors.New("err"))
		ret := box.fetchCameraSettingsFromNvr(cam)
		assert.Nil(t, ret)
	})
	t.Run("get osd settings err", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(&uniview.VideoStreamInfos{
			Num:              0,
			VideoStreamInfos: []uniview.VideoStreamInfo{},
		}, nil)
		nvrCli.EXPECT().GetChannelMediaVideoCapabilities(uint32(1)).Return(&uniview.MediaVideoCapabilities{}, nil)
		nvrCli.EXPECT().GetAudioDecodeStatuses(uint32(1)).Return([]uniview.AudioStatus{}, nil)
		nvrCli.EXPECT().GetOSDSettings(uint32(1)).Return(nil, errors.New("err"))
		ret := box.fetchCameraSettingsFromNvr(cam)
		assert.Nil(t, ret)
	})
	t.Run("get osd capabilities err", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(&uniview.VideoStreamInfos{
			Num:              0,
			VideoStreamInfos: []uniview.VideoStreamInfo{},
		}, nil)
		nvrCli.EXPECT().GetChannelMediaVideoCapabilities(uint32(1)).Return(&uniview.MediaVideoCapabilities{}, nil)
		nvrCli.EXPECT().GetAudioDecodeStatuses(uint32(1)).Return([]uniview.AudioStatus{}, nil)
		nvrCli.EXPECT().GetOSDSettings(uint32(1)).Return(&uniview.OSDSetting{}, nil)
		nvrCli.EXPECT().GetOSDCapabilities(uint32(1)).Return(nil, errors.New("err"))
		ret := box.fetchCameraSettingsFromNvr(cam)
		assert.Nil(t, ret)
	})
	t.Run("get audio input err", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(&uniview.VideoStreamInfos{
			Num:              0,
			VideoStreamInfos: []uniview.VideoStreamInfo{},
		}, nil)
		nvrCli.EXPECT().GetChannelMediaVideoCapabilities(uint32(1)).Return(&uniview.MediaVideoCapabilities{}, nil)
		nvrCli.EXPECT().GetAudioDecodeStatuses(uint32(1)).Return([]uniview.AudioStatus{}, nil)
		nvrCli.EXPECT().GetOSDSettings(uint32(1)).Return(&uniview.OSDSetting{}, nil)
		nvrCli.EXPECT().GetOSDCapabilities(uint32(1)).Return(&uniview.OSDCapabilities{}, nil)
		nvrCli.EXPECT().GetChannelMediaAudioInput(uint32(1)).Return(nil, errors.New("err"))
		ret := box.fetchCameraSettingsFromNvr(cam)
		assert.Nil(t, ret)
	})
	t.Run("get record schedule err", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(&uniview.VideoStreamInfos{
			Num:              0,
			VideoStreamInfos: []uniview.VideoStreamInfo{},
		}, nil)
		nvrCli.EXPECT().GetChannelMediaVideoCapabilities(uint32(1)).Return(&uniview.MediaVideoCapabilities{}, nil)
		nvrCli.EXPECT().GetAudioDecodeStatuses(uint32(1)).Return([]uniview.AudioStatus{}, nil)
		nvrCli.EXPECT().GetOSDSettings(uint32(1)).Return(&uniview.OSDSetting{}, nil)
		nvrCli.EXPECT().GetOSDCapabilities(uint32(1)).Return(&uniview.OSDCapabilities{}, nil)
		nvrCli.EXPECT().GetChannelMediaAudioInput(uint32(1)).Return(&uniview.AudioInputCfg{}, nil)
		nvrCli.EXPECT().GetChannelStorageScheduleRecord(uint32(1)).Return(nil, errors.New("err"))

		ret := box.fetchCameraSettingsFromNvr(cam)
		assert.Nil(t, ret)
	})
	t.Run("current camera setting nil 1", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(&uniview.VideoStreamInfos{
			Num:              0,
			VideoStreamInfos: []uniview.VideoStreamInfo{},
		}, nil)
		nvrCli.EXPECT().GetChannelMediaVideoCapabilities(uint32(1)).Return(&uniview.MediaVideoCapabilities{}, nil)
		nvrCli.EXPECT().GetAudioDecodeStatuses(uint32(1)).Return([]uniview.AudioStatus{}, nil)
		nvrCli.EXPECT().GetOSDSettings(uint32(1)).Return(&uniview.OSDSetting{}, nil)
		nvrCli.EXPECT().GetOSDCapabilities(uint32(1)).Return(&uniview.OSDCapabilities{}, nil)
		nvrCli.EXPECT().GetChannelMediaAudioInput(uint32(1)).Return(&uniview.AudioInputCfg{}, nil)
		nvrCli.EXPECT().GetChannelStorageScheduleRecord(uint32(1)).Return(&uniview.RecordScheduleInfo{}, nil)
		cam.EXPECT().GetSN().Return("cam_sn").Times(1)
		cam.EXPECT().GetID().Return(10).Times(2)
		ret := box.fetchCameraSettingsFromNvr(cam)
		assert.NotNil(t, ret)
	})
	t.Run("current camera setting nil 2", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(&uniview.VideoStreamInfos{
			Num:              0,
			VideoStreamInfos: []uniview.VideoStreamInfo{},
		}, nil)
		nvrCli.EXPECT().GetChannelMediaVideoCapabilities(uint32(1)).Return(&uniview.MediaVideoCapabilities{}, nil)
		nvrCli.EXPECT().GetAudioDecodeStatuses(uint32(1)).Return([]uniview.AudioStatus{}, nil)
		nvrCli.EXPECT().GetOSDSettings(uint32(1)).Return(&uniview.OSDSetting{}, nil)
		nvrCli.EXPECT().GetOSDCapabilities(uint32(1)).Return(&uniview.OSDCapabilities{}, nil)
		nvrCli.EXPECT().GetChannelMediaAudioInput(uint32(1)).Return(&uniview.AudioInputCfg{}, nil)
		nvrCli.EXPECT().GetChannelStorageScheduleRecord(uint32(1)).Return(&uniview.RecordScheduleInfo{}, nil)
		cam.EXPECT().GetSN().Return("cam_sn").Times(1)
		cam.EXPECT().GetID().Return(10).Times(2)
		ret := box.fetchCameraSettingsFromNvr(cam)
		expectSetting := &cloud.CameraSettings{
			CamID:             10,
			CamSN:             "cam_sn",
			CloudEventTypes:   []string{}, // api in cloud will ignore this param, so just give []string{}
			CloudEventMeta:    &cloud.CloudEventMeta{},
			CloudStorageMeta:  &cloud.CloudStorageMeta{},
			StreamSettings:    []uniview.VideoStreamInfo{},
			VideoCapabilities: &uniview.MediaVideoCapabilities{},
			AudioSettings:     []uniview.AudioStatus{},
			OSDSettings:       &uniview.OSDSetting{},
			OSDCapabilities:   &uniview.OSDCapabilities{},
			AudioInput:        &uniview.AudioInputCfg{},
			RecordSchedule:    &uniview.RecordScheduleInfo{},
		}
		assert.NotNil(t, ret)
		assert.Equal(t, expectSetting, ret)
	})

	t.Run("audio nvr version not required", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(&uniview.VideoStreamInfos{
			Num:              0,
			VideoStreamInfos: []uniview.VideoStreamInfo{},
		}, nil)
		nvrCli.EXPECT().GetChannelMediaVideoCapabilities(uint32(1)).Return(&uniview.MediaVideoCapabilities{}, nil)
		nvrCli.EXPECT().GetAudioDecodeStatuses(uint32(1)).Return([]uniview.AudioStatus{}, nil)
		nvrCli.EXPECT().GetOSDSettings(uint32(1)).Return(&uniview.OSDSetting{}, nil)
		nvrCli.EXPECT().GetOSDCapabilities(uint32(1)).Return(&uniview.OSDCapabilities{}, nil)
		nvrCli.EXPECT().GetChannelMediaAudioInput(uint32(1)).Return(nil, uniview.ErrorInvalidLAPI)
		nvrCli.EXPECT().GetChannelStorageScheduleRecord(uint32(1)).Return(nil, uniview.ErrorInvalidLAPI)
		cam.EXPECT().GetSN().Return("cam_sn").Times(1)
		cam.EXPECT().GetID().Return(10).Times(2)
		expectSetting := &cloud.CameraSettings{
			CamID:             10,
			CamSN:             "cam_sn",
			CloudEventTypes:   []string{}, // api in cloud will ignore this param, so just give []string{}
			CloudEventMeta:    &cloud.CloudEventMeta{},
			StreamSettings:    []uniview.VideoStreamInfo{},
			VideoCapabilities: &uniview.MediaVideoCapabilities{},
			AudioSettings:     []uniview.AudioStatus{},
			OSDSettings:       &uniview.OSDSetting{},
			OSDCapabilities:   &uniview.OSDCapabilities{},
			AudioInput:        nil,
			RecordSchedule:    nil,
		}
		cloud.SaveCameraSettings([]*cloud.CameraSettings{expectSetting})
		ret := box.fetchCameraSettingsFromNvr(cam)
		assert.Nil(t, ret)
	})

	t.Run("setting not change", func(t *testing.T) {
		init()
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(&uniview.VideoStreamInfos{
			Num:              0,
			VideoStreamInfos: []uniview.VideoStreamInfo{},
		}, nil)
		nvrCli.EXPECT().GetChannelMediaVideoCapabilities(uint32(1)).Return(&uniview.MediaVideoCapabilities{}, nil)
		nvrCli.EXPECT().GetAudioDecodeStatuses(uint32(1)).Return([]uniview.AudioStatus{}, nil)
		nvrCli.EXPECT().GetOSDSettings(uint32(1)).Return(&uniview.OSDSetting{}, nil)
		nvrCli.EXPECT().GetOSDCapabilities(uint32(1)).Return(&uniview.OSDCapabilities{}, nil)
		nvrCli.EXPECT().GetChannelMediaAudioInput(uint32(1)).Return(&uniview.AudioInputCfg{}, nil)
		nvrCli.EXPECT().GetChannelStorageScheduleRecord(uint32(1)).Return(&uniview.RecordScheduleInfo{}, nil)
		cam.EXPECT().GetSN().Return("cam_sn").Times(1)
		cam.EXPECT().GetID().Return(10).Times(2)
		expectSetting := &cloud.CameraSettings{
			CamID:             10,
			CamSN:             "cam_sn",
			CloudEventTypes:   []string{}, // api in cloud will ignore this param, so just give []string{}
			CloudEventMeta:    &cloud.CloudEventMeta{},
			StreamSettings:    []uniview.VideoStreamInfo{},
			VideoCapabilities: &uniview.MediaVideoCapabilities{},
			AudioSettings:     []uniview.AudioStatus{},
			OSDSettings:       &uniview.OSDSetting{},
			OSDCapabilities:   &uniview.OSDCapabilities{},
			AudioInput:        &uniview.AudioInputCfg{},
			RecordSchedule:    &uniview.RecordScheduleInfo{},
		}
		cloud.SaveCameraSettings([]*cloud.CameraSettings{expectSetting})
		ret := box.fetchCameraSettingsFromNvr(cam)
		assert.Nil(t, ret)
	})
}

func Test_updateRemoteCameraSettings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	var nvrMock *mock.MockNVRManager
	var cam *mock.MockAICamera
	var box *baseBox
	var cfg *mock.MockConfig
	var cg *mock.MockCamGroup

	init := func() {
		nvrMock = mock.NewMockNVRManager(ctrl)
		cam = mock.NewMockAICamera(ctrl)
		cfg = mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		box = New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		box.nvrManager = nvrMock
		cg = mock.NewMockCamGroup(ctrl)
		box.camGroup = cg
		cloud.ClearCameraSettings()
	}

	t.Run("camera list length 0", func(t *testing.T) {
		init()
		cg.EXPECT().AllCameras().Return([]base.Camera{}).Times(1)
		box.updateRemoteCameraSettings()
	})

	t.Run("camera not online", func(t *testing.T) {
		init()
		cg.EXPECT().AllCameras().Return([]base.Camera{cam}).Times(1)
		cam.EXPECT().GetOnline().Return(false).Times(1)
		box.updateRemoteCameraSettings()
	})

	t.Run("camera not plugged", func(t *testing.T) {
		init()
		cg.EXPECT().AllCameras().Return([]base.Camera{cam}).Times(1)
		cam.EXPECT().GetOnline().Return(true).Times(1)
		nvrMock.EXPECT().IsCameraPlugged(cam).Return(false).Times(1)
		box.updateRemoteCameraSettings()
	})
	t.Run("fetch camera setting ok", func(t *testing.T) {
		init()
		cg.EXPECT().AllCameras().Return([]base.Camera{cam}).Times(2)
		cam.EXPECT().GetOnline().Return(true).Times(1)
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		cam.EXPECT().GetBrand().Times(1)
		nvrMock.EXPECT().IsCameraPlugged(cam).Return(true).Times(1)
		nvrCli := uniview.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nvrCli, nil)
		cam.EXPECT().GetChannel().Return(uint32(1)).Times(1)
		nvrCli.EXPECT().GetChannelStreamDetailInfo(uint32(1)).Return(&uniview.VideoStreamInfos{
			Num:              0,
			VideoStreamInfos: []uniview.VideoStreamInfo{},
		}, nil)
		nvrCli.EXPECT().GetChannelMediaVideoCapabilities(uint32(1)).Return(&uniview.MediaVideoCapabilities{}, nil)
		nvrCli.EXPECT().GetAudioDecodeStatuses(uint32(1)).Return([]uniview.AudioStatus{}, nil)
		nvrCli.EXPECT().GetOSDSettings(uint32(1)).Return(&uniview.OSDSetting{}, nil)
		nvrCli.EXPECT().GetOSDCapabilities(uint32(1)).Return(&uniview.OSDCapabilities{}, nil)
		nvrCli.EXPECT().GetChannelMediaAudioInput(uint32(1)).Return(&uniview.AudioInputCfg{}, nil)
		nvrCli.EXPECT().GetChannelStorageScheduleRecord(uint32(1)).Return(&uniview.RecordScheduleInfo{}, nil)
		cam.EXPECT().GetSN().Return("cam_sn").Times(1)
		cam.EXPECT().GetID().Return(10).Times(2)
		expectSetting := &cloud.CameraSettings{
			CamID:             10,
			CamSN:             "cam_sn",
			CloudEventTypes:   []string{}, // api in cloud will ignore this param, so just give []string{}
			CloudEventMeta:    &cloud.CloudEventMeta{},
			CloudStorageMeta:  &cloud.CloudStorageMeta{},
			StreamSettings:    []uniview.VideoStreamInfo{},
			VideoCapabilities: &uniview.MediaVideoCapabilities{},
			AudioSettings:     []uniview.AudioStatus{},
			OSDSettings:       &uniview.OSDSetting{},
			OSDCapabilities:   &uniview.OSDCapabilities{},
			AudioInput:        &uniview.AudioInputCfg{},
			RecordSchedule:    &uniview.RecordScheduleInfo{},
		}

		cloudClient := mock.NewMockClient(ctrl)
		box.apiClient = cloudClient
		cloudClient.EXPECT().UploadCameraSettings([]*cloud.CameraSettings{expectSetting}).Return(nil).Times(1)
		box.updateRemoteCameraSettings()
	})
}

func Test_updateLocalCameraSettings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	var nvrMock *mock.MockNVRManager
	var cam *mock.MockAICamera
	var box *baseBox
	var cfg *mock.MockConfig
	var cg *mock.MockCamGroup

	init := func() {
		nvrMock = mock.NewMockNVRManager(ctrl)
		cam = mock.NewMockAICamera(ctrl)
		cfg = mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		box = New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		box.nvrManager = nvrMock
		cg = mock.NewMockCamGroup(ctrl)
		box.camGroup = cg
		cloud.ClearCameraSettings()
		mockSetting := &cloud.CameraSettings{
			CamID:             10,
			CamSN:             "cam_sn",
			CloudEventTypes:   []string{}, // api in cloud will ignore this param, so just give []string{}
			CloudEventMeta:    &cloud.CloudEventMeta{},
			StreamSettings:    []uniview.VideoStreamInfo{},
			VideoCapabilities: &uniview.MediaVideoCapabilities{},
			AudioSettings:     []uniview.AudioStatus{},
			OSDSettings:       &uniview.OSDSetting{},
			OSDCapabilities:   &uniview.OSDCapabilities{},
		}
		cloud.SaveCameraSettings([]*cloud.CameraSettings{mockSetting})
	}

	t.Run("no cam", func(t *testing.T) {
		init()
		cg.EXPECT().AllCameras().Return([]base.Camera{}).Times(1)
		box.updateLocalCameraSettings()
		assert.Nil(t, cloud.GetCameraSettingsByID(10))
	})

	t.Run("no ai cam", func(t *testing.T) {
		init()
		thermalCam := mock.NewMockThermal1Camera(ctrl)
		cg.EXPECT().AllCameras().Return([]base.Camera{thermalCam}).Times(1)
		box.updateLocalCameraSettings()
		assert.Nil(t, cloud.GetCameraSettingsByID(10))
	})

	t.Run("no plugged cam", func(t *testing.T) {
		init()
		cg.EXPECT().AllCameras().Return([]base.Camera{cam}).Times(1)
		cam.EXPECT().GetOnline().Return(true).Times(1)
		nvrMock.EXPECT().IsCameraPlugged(cam).Return(false).Times(1)
		box.updateLocalCameraSettings()
		assert.Nil(t, cloud.GetCameraSettingsByID(10))
	})

	t.Run("no online cam", func(t *testing.T) {
		init()
		cg.EXPECT().AllCameras().Return([]base.Camera{cam}).Times(1)
		cam.EXPECT().GetOnline().Return(false).Times(1)
		box.updateLocalCameraSettings()
		assert.Nil(t, cloud.GetCameraSettingsByID(10))
	})

	t.Run("get camera setting from cloud api", func(t *testing.T) {
		init()
		cg.EXPECT().AllCameras().Return([]base.Camera{cam}).AnyTimes()
		nvrMock.EXPECT().IsCameraPlugged(cam).Return(true).Times(1)
		cam.EXPECT().GetOnline().Return(true).AnyTimes()
		cam.EXPECT().GetID().Return(10).AnyTimes()
		apiCli := mock.NewMockClient(ctrl)
		box.apiClient = apiCli
		remoteSetting := &cloud.CameraSettings{
			CamID:           10,
			CamSN:           "cam_sn",
			CloudEventTypes: []string{"car"}, // api in cloud will ignore this param, so just give []string{}
			AudioSettings:   []uniview.AudioStatus{{StreamID: 1, IsDecodeAudio: 1}},
		}
		apiCli.EXPECT().GetCameraSettings([]int{10}).Return([]*cloud.CameraSettings{remoteSetting}, nil).Times(1)
		box.updateLocalCameraSettings()
		assert.NotNil(t, cloud.GetCameraSettingsByID(10))
		expectSetting := &cloud.CameraSettings{
			CamID:             10,
			CamSN:             "cam_sn",
			CloudEventTypes:   []string{"car"}, // api in cloud will ignore this param, so just give []string{}
			CloudEventMeta:    &cloud.CloudEventMeta{},
			StreamSettings:    []uniview.VideoStreamInfo{},
			VideoCapabilities: &uniview.MediaVideoCapabilities{},
			AudioSettings:     []uniview.AudioStatus{{StreamID: 1, IsDecodeAudio: 1}},
			OSDSettings:       &uniview.OSDSetting{},
			OSDCapabilities:   &uniview.OSDCapabilities{},
		}
		assert.Equal(t, expectSetting, cloud.GetCameraSettingsByID(10))
	})
}

func TestCheckIotDevice(t *testing.T) {
	t.Run("miss", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.arpSearcher = arpSearcher
		cfg.EXPECT().GetArpReportInterval().Return(uint16(1)).Times(1)
		cfg.EXPECT().GetArpReportTimes().Return(uint16(1)).Times(1)
		mockDeviceMap := make(map[string]arp.Device)
		arpSearcher.EXPECT().GetDevices().Return(mockDeviceMap).Times(1)
		inputParam := make(map[string]*cloud.IotDevice)
		inputParam["xx:xx:xx:xx:xx"] = &cloud.IotDevice{
			MacAddress: "xx:xx:xx:xx:xx",
			IpAddress:  "192.168.0.1",
		}
		assert.True(t, box.checkIotDevice(inputParam))
	})

	t.Run("found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.arpSearcher = arpSearcher
		cfg.EXPECT().GetArpReportInterval().Return(uint16(1)).Times(1)
		cfg.EXPECT().GetArpReportTimes().Return(uint16(1)).Times(1)
		mockDeviceMap := make(map[string]arp.Device)
		mockDeviceMap["xx:xx:xx:xx:xx"] = arp.Device{
			Mac: "xx:xx:xx:xx:xx",
			IP:  "192.168.0.1",
		}
		arpSearcher.EXPECT().GetDevices().Return(mockDeviceMap).Times(1)
		inputParam := make(map[string]*cloud.IotDevice)
		inputParam["xx:xx:xx:xx:xx"] = &cloud.IotDevice{
			MacAddress: "xx:xx:xx:xx:xx",
			IpAddress:  "192.168.0.1",
		}
		assert.False(t, box.checkIotDevice(inputParam))
	})

	t.Run("1 found 1 miss", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.arpSearcher = arpSearcher
		cfg.EXPECT().GetArpReportInterval().Return(uint16(1)).Times(1)
		cfg.EXPECT().GetArpReportTimes().Return(uint16(1)).Times(1)
		mockDeviceMap := make(map[string]arp.Device)
		mockDeviceMap["xx:xx:xx:xx:xx"] = arp.Device{
			Mac: "xx:xx:xx:xx:xx",
			IP:  "192.168.0.1",
		}
		arpSearcher.EXPECT().GetDevices().Return(mockDeviceMap).Times(1)
		inputParam := make(map[string]*cloud.IotDevice)
		inputParam["xx:xx:xx:xx:xx"] = &cloud.IotDevice{
			MacAddress: "xx:xx:xx:xx:xx",
			IpAddress:  "192.168.0.1",
		}
		inputParam["yy:yy:yy:yy:yy"] = &cloud.IotDevice{
			MacAddress: "yy:yy:yy:yy:yy",
			IpAddress:  "192.168.0.2",
		}
		assert.True(t, box.checkIotDevice(inputParam))
	})

	t.Run("1 found 1 update", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.arpSearcher = arpSearcher
		cfg.EXPECT().GetArpReportInterval().Return(uint16(1)).Times(1)
		cfg.EXPECT().GetArpReportTimes().Return(uint16(1)).Times(1)
		mockDeviceMap := make(map[string]arp.Device)
		mockDeviceMap["xx:xx:xx:xx:xx"] = arp.Device{
			Mac: "xx:xx:xx:xx:xx",
			IP:  "192.168.0.1",
		}
		arpSearcher.EXPECT().GetDevices().Return(mockDeviceMap).Times(1)
		inputParam := make(map[string]*cloud.IotDevice)
		inputParam["xx:xx:xx:xx:xx"] = &cloud.IotDevice{
			MacAddress: "xx:xx:xx:xx:xx",
			IpAddress:  "192.168.0.1",
		}
		inputParam["yy:yy:yy:yy:yy"] = &cloud.IotDevice{
			MacAddress: "yy:yy:yy:yy:yy",
			IpAddress:  "192.168.0.2",
			Updated:    true,
		}
		assert.False(t, box.checkIotDevice(inputParam))
	})

	t.Run("all try", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.arpSearcher = arpSearcher
		cfg.EXPECT().GetArpReportInterval().Return(uint16(1)).Times(1)
		cfg.EXPECT().GetArpReportTimes().Return(uint16(2)).Times(1)
		mockDeviceMap := make(map[string]arp.Device)
		arpSearcher.EXPECT().GetDevices().Return(mockDeviceMap).Times(2)
		inputParam := make(map[string]*cloud.IotDevice)
		inputParam["xx:xx:xx:xx:xx"] = &cloud.IotDevice{
			MacAddress: "xx:xx:xx:xx:xx",
			IpAddress:  "192.168.0.1",
		}
		inputParam["yy:yy:yy:yy:yy"] = &cloud.IotDevice{
			MacAddress: "yy:yy:yy:yy:yy",
			IpAddress:  "192.168.0.2",
			Updated:    true,
		}
		assert.True(t, box.checkIotDevice(inputParam))
	})

	t.Run("1 try", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.arpSearcher = arpSearcher
		cfg.EXPECT().GetArpReportInterval().Return(uint16(1)).Times(1)
		cfg.EXPECT().GetArpReportTimes().Return(uint16(2)).Times(1)
		mockDeviceMap := make(map[string]arp.Device)
		mockDeviceMap["xx:xx:xx:xx:xx"] = arp.Device{
			Mac: "xx:xx:xx:xx:xx",
			IP:  "192.168.0.1",
		}
		mockDeviceMap["yy:yy:yy:yy:yy"] = arp.Device{
			Mac: "yy:yy:yy:yy:yy",
			IP:  "192.168.0.3",
		}
		arpSearcher.EXPECT().GetDevices().Return(mockDeviceMap).Times(1)
		inputParam := make(map[string]*cloud.IotDevice)
		inputParam["xx:xx:xx:xx:xx"] = &cloud.IotDevice{
			MacAddress: "xx:xx:xx:xx:xx",
			IpAddress:  "192.168.0.1",
		}
		inputParam["yy:yy:yy:yy:yy"] = &cloud.IotDevice{
			MacAddress: "yy:yy:yy:yy:yy",
			IpAddress:  "192.168.0.2",
		}
		assert.False(t, box.checkIotDevice(inputParam))
		assert.Equal(t, "192.168.0.3", inputParam["yy:yy:yy:yy:yy"].IpAddress)
		assert.Equal(t, true, inputParam["yy:yy:yy:yy:yy"].Updated)
	})
}

func TestUpdateIotDevices(t *testing.T) {
	t.Run("arp disable", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		cfg.EXPECT().GetArpEnable().Return(false).Times(1)
		box.updateIotDevices()
	})

	t.Run("api client error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		cfg.EXPECT().GetArpEnable().Return(true).Times(1)
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		apiClient := mock.NewMockClient(ctrl)
		box.apiClient = apiClient
		apiClient.EXPECT().GetIotDevices().Return(nil, errors.New("err iot devices")).Times(1)

		box.updateIotDevices()
	})

	t.Run("api client error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		cfg.EXPECT().GetArpEnable().Return(true).Times(1)
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		apiClient := mock.NewMockClient(ctrl)
		box.apiClient = apiClient

		apiClient.EXPECT().GetIotDevices().Return(nil, errors.New("err iot devices")).Times(1)

		box.updateIotDevices()
	})

	t.Run("cached", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		cfg.EXPECT().GetArpEnable().Return(true).Times(1)
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		apiClient := mock.NewMockClient(ctrl)
		box.apiClient = apiClient
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.arpSearcher = arpSearcher

		device1 := &cloud.IotDevice{
			MacAddress: "xx:xx:xx:xx:xx",
			IpAddress:  "192.168.0.1",
		}
		device2 := &cloud.IotDevice{
			MacAddress: "yy:yy:yy:yy:yy",
			IpAddress:  "192.168.0.2",
		}
		apiClient.EXPECT().GetIotDevices().Return([]*cloud.IotDevice{device1, device2}, nil).Times(1)

		mockDeviceMap := make(map[string]arp.Device)
		mockDeviceMap["xx:xx:xx:xx:xx"] = arp.Device{
			Mac: "xx:xx:xx:xx:xx",
			IP:  "192.168.0.1",
		}
		mockDeviceMap["yy:yy:yy:yy:yy"] = arp.Device{
			Mac: "yy:yy:yy:yy:yy",
			IP:  "192.168.0.2",
		}
		arpSearcher.EXPECT().GetDevices().Return(mockDeviceMap).Times(1)
		arpSearcher.EXPECT().CacheExpired().Return(false).Times(1)
		cfg.EXPECT().GetArpReportInterval().Return(uint16(1)).Times(1)
		cfg.EXPECT().GetArpReportTimes().Return(uint16(1)).Times(1)

		apiClient.EXPECT().UploadIotDevices([]*cloud.IotDevice{device1, device2}).Return(nil).Times(1)

		box.updateIotDevices()
		assert.Equal(t, true, device1.Updated)
		assert.Equal(t, true, device2.Updated)
	})

	t.Run("cache expire, all found when search device", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		cfg.EXPECT().GetArpEnable().Return(true).Times(1)
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		apiClient := mock.NewMockClient(ctrl)
		box.apiClient = apiClient
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.arpSearcher = arpSearcher

		device1 := &cloud.IotDevice{
			MacAddress: "xx:xx:xx:xx:xx",
			IpAddress:  "192.168.0.1",
		}
		device2 := &cloud.IotDevice{
			MacAddress: "yy:yy:yy:yy:yy",
			IpAddress:  "192.168.0.2",
		}
		apiClient.EXPECT().GetIotDevices().Return([]*cloud.IotDevice{device1, device2}, nil).Times(1)

		mockDeviceMap := make(map[string]arp.Device)
		mockDeviceMap["xx:xx:xx:xx:xx"] = arp.Device{
			Mac: "xx:xx:xx:xx:xx",
			IP:  "192.168.0.1",
		}
		mockDeviceMap["yy:yy:yy:yy:yy"] = arp.Device{
			Mac: "yy:yy:yy:yy:yy",
			IP:  "192.168.0.2",
		}
		arpSearcher.EXPECT().GetDevices().Return(mockDeviceMap).Times(1)
		arpSearcher.EXPECT().CacheExpired().Return(true).Times(1)
		arpSearcher.EXPECT().SearchDevices([]string{"192.168.0.1", "192.168.0.2"}, []string{"xx:xx:xx:xx:xx", "yy:yy:yy:yy:yy"}).Times(1)
		cfg.EXPECT().GetArpReportInterval().Return(uint16(1)).Times(1)
		cfg.EXPECT().GetArpReportTimes().Return(uint16(1)).Times(1)

		apiClient.EXPECT().UploadIotDevices([]*cloud.IotDevice{device1, device2}).Return(nil).Times(1)

		box.updateIotDevices()
		assert.Equal(t, true, device1.Updated)
		assert.Equal(t, true, device2.Updated)
	})

	t.Run("cache expire, device1 miss when search device", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		cfg.EXPECT().GetArpEnable().Return(true).Times(1)
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		apiClient := mock.NewMockClient(ctrl)
		apiClient.EXPECT().UploadAlarmInfo(gomock.Any()).Return(nil).AnyTimes()
		box.apiClient = apiClient
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.arpSearcher = arpSearcher

		device1 := &cloud.IotDevice{
			MacAddress: "xx:xx:xx:xx:xx",
			IpAddress:  "192.168.0.1",
			State:      utils.DeviceStatusOnline,
		}
		device2 := &cloud.IotDevice{
			MacAddress: "yy:yy:yy:yy:yy",
			IpAddress:  "192.168.0.2",
			State:      utils.DeviceStatusOnline,
		}
		apiClient.EXPECT().GetIotDevices().Return([]*cloud.IotDevice{device1, device2}, nil).Times(1)

		mockDeviceMap := make(map[string]arp.Device)
		mockDeviceMap["yy:yy:yy:yy:yy"] = arp.Device{
			Mac: "yy:yy:yy:yy:yy",
			IP:  "192.168.0.2",
		}
		arpSearcher.EXPECT().GetDevices().Return(mockDeviceMap).Times(2)
		arpSearcher.EXPECT().CacheExpired().Return(true).Times(1)
		arpSearcher.EXPECT().SearchDevices([]string{"192.168.0.1", "192.168.0.2"}, []string{"xx:xx:xx:xx:xx", "yy:yy:yy:yy:yy"}).Times(1)
		arpSearcher.EXPECT().SearchAll().Times(1)
		cfg.EXPECT().GetArpReportInterval().Return(uint16(1)).Times(2)
		cfg.EXPECT().GetArpReportTimes().Return(uint16(1)).Times(2)

		apiClient.EXPECT().UploadIotDevices([]*cloud.IotDevice{device1, device2}).Return(nil).Times(1)

		box.updateIotDevices()
		assert.Equal(t, false, device1.Updated)
		assert.Equal(t, utils.DeviceStatusOffline, device1.State)
		assert.Equal(t, true, device2.Updated)
	})

	t.Run("iot device offline", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := mock.NewMockConfig(ctrl)
		cfg.EXPECT().GetCameraTimeoutSecs().Return(60).AnyTimes()
		cfg.EXPECT().GetArpEnable().Return(true).Times(1)
		box := New(cfg, configs.BoxInfo{}, nil).(*baseBox)
		apiClient := mock.NewMockClient(ctrl)
		box.apiClient = apiClient
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.arpSearcher = arpSearcher

		device1 := &cloud.IotDevice{
			MacAddress: "xx:xx:xx:xx:xx",
			IpAddress:  "192.168.0.1",
			State:      utils.DeviceStatusOnline,
		}
		apiClient.EXPECT().GetIotDevices().Return([]*cloud.IotDevice{device1}, nil).Times(1)

		mockDeviceMap := make(map[string]arp.Device)
		arpSearcher.EXPECT().GetDevices().Return(mockDeviceMap).Times(1)
		arpSearcher.EXPECT().CacheExpired().Return(false).Times(1)
		cfg.EXPECT().GetArpReportInterval().Return(uint16(1)).Times(1)
		cfg.EXPECT().GetArpReportTimes().Return(uint16(1)).Times(1)

		apiClient.EXPECT().UploadAlarmInfo(gomock.Any()).Return(nil).MaxTimes(1)
		apiClient.EXPECT().UploadIotDevices([]*cloud.IotDevice{device1}).Return(nil).Times(1)

		box.updateIotDevices()
		assert.Equal(t, false, device1.Updated)
		assert.Equal(t, utils.DeviceStatusOffline, device1.State)
	})
}
