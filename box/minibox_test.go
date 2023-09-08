package box

import (
	"errors"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/turingvideo/turing-common/log"

	"github.com/turingvideo/minibox/camera/base"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/configs"
	"github.com/turingvideo/minibox/mock"
	"github.com/turingvideo/minibox/utils"
)

func TestGetSetConfig(t *testing.T) {
	cfg := configs.NewEmptyConfig()
	info := configs.BoxInfo{}

	b := New(&cfg, info, nil)

	cfg.SetEventSavedHours(100)
	cfg.SetTemperatureUnit(configs.FUnit)
	cfg.SetTimeZone("America/Los_Angeles")

	assert.Equal(t, 100, b.GetConfig().GetEventSavedHours())
	assert.Equal(t, configs.FUnit, b.GetConfig().GetTemperatureUnit())
	assert.Equal(t, "America/Los_Angeles", b.GetConfig().GetTimeZone())
}

func TestUpdateAPIClientConfig(t *testing.T) {
	t.Run("client is nil", func(t *testing.T) {
		cfg := configs.NewEmptyConfig()
		b := New(&cfg, configs.BoxInfo{}, nil)

		assert.NotNil(t, b.UpdateAPIClientConfig())
	})

	t.Run("bad server url", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewEmptyConfig()
		b := New(&cfg, configs.BoxInfo{}, nil)

		assert.NotNil(t, b.UpdateAPIClientConfig())
	})

	t.Run("updates the config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		cfg := configs.NewTestConfig(nil, configs.LogConfig{}, configs.RedisConfig{}, configs.BoxConfig{
			APIServer: "http://google.com",
		})

		client := mock.NewMockClient(ctrl)

		b := baseBox{
			config:    &cfg,
			apiClient: client,
			configMux: &sync.Mutex{},
			boxInfo:   &configs.BoxInfo{},
		}

		client.EXPECT().UpdateClientConfig(gomock.Any())

		assert.Nil(t, b.UpdateAPIClientConfig())
	})
}

func TestInitCameras(t *testing.T) {
	t.Run("err with no client", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		b := New(&configs.BaseConfig{}, configs.BoxInfo{}, nil).(*baseBox)
		b.logger = zerolog.New(zerolog.NewConsoleWriter())
		b.apiClient = nil
		cg := mock.NewMockCamGroup(ctrl)
		cg.EXPECT().AllCameras().Return([]base.Camera{}).AnyTimes()
		b.camGroup = cg
		nvrManage := mock.NewMockNVRManager(ctrl)
		b.nvrManager = nvrManage
		nvrManage.EXPECT().GetNVRClientBySN(gomock.Any()).Return(nil, errors.New("nvr manager not ready")).AnyTimes()

		assert.Error(t, b.updateCameras())
	})

	t.Run("err with cloud err", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client := mock.NewMockClient(ctrl)
		b := New(&configs.BaseConfig{}, configs.BoxInfo{}, nil).(*baseBox)
		b.logger = zerolog.New(zerolog.NewConsoleWriter())
		b.apiClient = client

		cg := mock.NewMockCamGroup(ctrl)
		cg.EXPECT().AllCameras().Return([]base.Camera{}).AnyTimes()
		b.camGroup = cg
		nvrManage := mock.NewMockNVRManager(ctrl)
		b.nvrManager = nvrManage
		nvrManage.EXPECT().GetNVRClientBySN(gomock.Any()).Return(nil, errors.New("nvr manager not ready")).AnyTimes()

		client.EXPECT().GetCameras().Return(nil, errors.New("bad connection"))

		assert.Error(t, b.updateCameras())
	})

	t.Run("ok with no cloud cameras", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := mock.NewMockConfig(ctrl)
		// config.EXPECT().GetAiCameraConfig().Return(&configs.AiCameraConfig{SnapDuration: 60 * 60}).AnyTimes()
		config.EXPECT().Logging().Return(log.Config{}).AnyTimes()
		config.EXPECT().GetCameraTimeoutSecs().Return(10).AnyTimes()
		config.EXPECT().GetBoxType().Return("mini_v1").AnyTimes()
		client := mock.NewMockClient(ctrl)
		b := New(config, configs.BoxInfo{}, nil).(*baseBox)
		b.logger = zerolog.New(zerolog.NewConsoleWriter())
		b.apiClient = client
		client.EXPECT().GetCameras().Return(nil, nil)

		cg := mock.NewMockCamGroup(ctrl)
		cg.EXPECT().AllCameras().Return([]base.Camera{}).AnyTimes()
		b.camGroup = cg

		assert.NoError(t, b.updateCameras())
	})

	t.Run("adds cameras from the cloud to the local camera set", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := mock.NewMockConfig(ctrl)
		// config.EXPECT().GetAiCameraConfig().Return(&configs.AiCameraConfig{SnapDuration: 60 * 60}).AnyTimes()
		config.EXPECT().Logging().Return(log.Config{}).AnyTimes()
		config.EXPECT().GetCameraTimeoutSecs().Return(10).AnyTimes()
		config.EXPECT().GetBoxType().Return("mini_v1").AnyTimes()

		client := mock.NewMockClient(ctrl)
		cg := mock.NewMockCamGroup(ctrl)
		c := mock.NewMockThermal1Camera(ctrl)

		b := New(config, configs.BoxInfo{}, nil).(*baseBox)
		b.logger = zerolog.New(zerolog.NewConsoleWriter())
		b.apiClient = client
		b.camGroup = cg

		client.EXPECT().GetCameras().Return([]*cloud.Camera{{
			ID: 1,
			SN: "test_sn",
		}}, nil)

		cg.EXPECT().GetCameraBySN(gomock.Eq("test_sn")).Return(nil, errors.New("not found"))
		cg.EXPECT().AddCamera(gomock.Any())
		cg.EXPECT().AllCameras().Return([]base.Camera{c}).AnyTimes()

		c.EXPECT().GetID().Return(1)
		c.EXPECT().GetSN().Return("").Times(1)
		c.EXPECT().GetBrand().Return(utils.Thermal1).AnyTimes()
		c.EXPECT().GetOnline().Return(false).AnyTimes()
		c.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
		assert.NoError(t, b.updateCameras())
	})

	t.Run("if camera is already present, updates the ID", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := mock.NewMockConfig(ctrl)
		// config.EXPECT().GetAiCameraConfig().Return(&configs.AiCameraConfig{SnapDuration: 60 * 60}).AnyTimes()
		config.EXPECT().Logging().Return(log.Config{}).AnyTimes()
		config.EXPECT().GetCameraTimeoutSecs().Return(10).AnyTimes()
		config.EXPECT().GetBoxType().Return("mini_v1").AnyTimes()

		client := mock.NewMockClient(ctrl)
		cg := mock.NewMockCamGroup(ctrl)
		c := mock.NewMockThermal1Camera(ctrl)

		b := New(config, configs.BoxInfo{}, nil).(*baseBox)
		b.logger = zerolog.New(zerolog.NewConsoleWriter())
		b.apiClient = client
		b.camGroup = cg

		client.EXPECT().GetCameras().Return([]*cloud.Camera{{
			ID: 1,
			SN: "test_sn",
		}}, nil)

		cg.EXPECT().GetCameraBySN(gomock.Eq("test_sn")).Return(c, nil).Times(1)
		cg.EXPECT().AllCameras().Return([]base.Camera{c}).AnyTimes()

		c.EXPECT().UpdateCamera(gomock.Eq("test_sn"), gomock.Eq(1))
		c.EXPECT().GetID().Return(1)
		c.EXPECT().GetSN().Return("test_sn").Times(1)
		c.EXPECT().GetBrand().Return(utils.Thermal1).AnyTimes()
		c.EXPECT().GetOnline().Return(false).AnyTimes()
		c.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
		assert.NoError(t, b.updateCameras())
	})

	t.Run("if a thermal camera exists locally, adds this camera to the cloud", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := mock.NewMockConfig(ctrl)
		// config.EXPECT().GetAiCameraConfig().Return(&configs.AiCameraConfig{SnapDuration: 60 * 60}).AnyTimes()
		config.EXPECT().Logging().Return(log.Config{}).AnyTimes()
		config.EXPECT().GetCameraTimeoutSecs().Return(10).AnyTimes()
		config.EXPECT().GetBoxType().Return("mini_v1").AnyTimes()

		client := mock.NewMockClient(ctrl)
		cg := mock.NewMockCamGroup(ctrl)
		c := mock.NewMockThermal1Camera(ctrl)

		b := New(config, configs.BoxInfo{}, nil).(*baseBox)
		b.logger = zerolog.New(zerolog.NewConsoleWriter())
		b.apiClient = client
		b.camGroup = cg

		client.EXPECT().GetCameras().Return(nil, nil)
		client.EXPECT().AddCamera(gomock.Eq("test_sn"), gomock.Eq(string(utils.Thermal1)), gomock.Eq("test_ip")).
			Return(&cloud.Camera{ID: 2, SN: "test_sn2"}, nil)
		cg.EXPECT().AllCameras().Return([]base.Camera{c}).AnyTimes()

		c.EXPECT().GetID().Return(0).AnyTimes()
		c.EXPECT().GetSN().Return("test_sn").AnyTimes()
		c.EXPECT().GetBrand().Return(utils.Thermal1).AnyTimes()
		c.EXPECT().GetIP().Return("test_ip")
		c.EXPECT().GetOnline().Return(false).AnyTimes()
		c.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
		// camera can be updated using a different SN
		c.EXPECT().UpdateCamera(gomock.Eq("test_sn2"), gomock.Eq(2))

		assert.NoError(t, b.updateCameras())
	})

	t.Run("if an aicamera exists locally, but not in the cloud, delete locally", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := mock.NewMockConfig(ctrl)
		config.EXPECT().GetCameraTimeoutSecs().Return(10).Times(1)

		client := mock.NewMockClient(ctrl)
		cg := mock.NewMockCamGroup(ctrl)
		c := mock.NewMockAICamera(ctrl)

		b := New(config, configs.BoxInfo{}, nil).(*baseBox)
		b.logger = zerolog.New(zerolog.NewConsoleWriter())
		b.apiClient = client
		b.camGroup = cg

		client.EXPECT().GetCameras().Return(nil, nil)
		client.EXPECT().AddCamera(gomock.Eq("test_sn"), gomock.Eq(string(utils.Thermal1)), gomock.Eq("test_ip")).
			Return(&cloud.Camera{ID: 2, SN: "test_sn2"}, nil)
		client.EXPECT().GetCameraSettings(gomock.Any()).Return(nil, errors.New("api client error")).AnyTimes()
		cg.EXPECT().DelCamera(gomock.Eq("test_sn")).Times(1)

		cg.EXPECT().AllCameras().Return([]base.Camera{c}).AnyTimes()

		c.EXPECT().GetID().Return(0).AnyTimes()
		c.EXPECT().GetSN().Return("test_sn").AnyTimes()
		c.EXPECT().GetBrand().Return(utils.Thermal1).AnyTimes()
		c.EXPECT().GetIP().Return("test_ip")
		c.EXPECT().GetOnline().Return(false).AnyTimes()
		c.EXPECT().GetManufacturer().Return(utils.TuringUniview).AnyTimes()
		// camera can be updated using a different SN
		c.EXPECT().UpdateCamera(gomock.Eq("test_sn2"), gomock.Eq(2))

		assert.NoError(t, b.updateCameras())
	})
}
