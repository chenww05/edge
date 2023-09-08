package box

import (
	"errors"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turingvideo/minibox/camera"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/configs"
	"github.com/turingvideo/minibox/mock"
	"github.com/turingvideo/minibox/utils"
)

type mockCamera struct {
	utils.BaseCamera
}

func (m mockCamera) Heartbeat(ip, version string) {
	m.BaseCamera.Heartbeat(ip)
}

func TestAddCamera(t *testing.T) {
	cfg := configs.NewEmptyConfig()
	b := New(&cfg, configs.BoxInfo{}, nil)

	t.Run("adds a camera", func(t *testing.T) {
		camera := mockCamera{
			BaseCamera: utils.BaseCamera{
				SN:  "test",
				Mux: &sync.Mutex{},
			},
		}
		b.AddCamera(&camera)

		c, err := b.GetCameraBySN("test")
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, &camera, c)
	})

	t.Run("updates camera ID", func(t *testing.T) {
		camera := mockCamera{
			BaseCamera: utils.BaseCamera{
				SN:  "test",
				Mux: &sync.Mutex{},
			},
		}

		b.AddCamera(&camera)

		c, err := b.GetCameraBySN("test")
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, &camera, c)

		c.UpdateCamera("test", 50)
		c, err = b.GetCameraBySN("test")
		if err != nil {
			t.Error(err)
		}
		// assert.Equal(t, &camera, c)
		assert.Equal(t, 50, c.GetID())
	})

	t.Run("works when online", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client := mock.NewMockClient(ctrl)
		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			apiClient: client,
			logger:    zerolog.New(zerolog.NewConsoleWriter()),
			camGroup:  camera.NewCamGroup(10),
		}

		camera := mockCamera{
			BaseCamera: utils.BaseCamera{
				SN:  "test_unique",
				Mux: &sync.Mutex{},
			},
		}

		client.EXPECT().AddCamera(gomock.Eq(camera.SN), gomock.Eq(string(camera.GetBrand())), gomock.Eq(camera.IP)).
			Return(&cloud.Camera{ID: 10}, nil)

		b.AddCamera(&camera)

		c, err := b.GetCameraBySN("test_unique")
		require.NoError(t, err)

		assert.Equal(t, &camera, c)
		assert.Equal(t, 10, c.GetID())
		assert.Equal(t, "test_unique", c.GetSN())
	})

	t.Run("works when cloud error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client := mock.NewMockClient(ctrl)
		b := baseBox{
			config:    &cfg,
			configMux: &sync.Mutex{},
			apiClient: client,
			logger:    zerolog.New(zerolog.NewConsoleWriter()),
			camGroup:  camera.NewCamGroup(10),
		}

		camera := mockCamera{
			BaseCamera: utils.BaseCamera{
				SN:  "test_unique2",
				Mux: &sync.Mutex{},
			},
		}

		client.EXPECT().AddCamera(gomock.Eq(camera.SN), gomock.Eq(string(camera.GetBrand())), gomock.Eq(camera.IP)).
			Return(nil, errors.New("cloud error"))

		b.AddCamera(&camera)

		c, err := b.GetCameraBySN("test_unique2")
		require.NoError(t, err)

		assert.Equal(t, &camera, c)
		assert.Equal(t, 0, c.GetID())
		assert.Equal(t, "test_unique2", c.GetSN())
	})
}
