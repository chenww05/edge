package box

import (
	"fmt"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/turingvideo/goshawk/uniview"

	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/configs"
	"github.com/turingvideo/minibox/mock"
)

func TestGetTokenFromCache(t *testing.T) {
	t.Run("cache is nil", func(t *testing.T) {
		t.Parallel()
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		b := box.(*baseBox)
		token := b.getTokenFromCache(TokenNameCameraVideo)
		assert.Nil(t, token)
	})

	t.Run("cache is has", func(t *testing.T) {
		t.Parallel()
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		b := box.(*baseBox)
		tk := cloud.Token{}
		b.setTokenInCache(TokenNameCameraVideo, tk)
		token := b.getTokenFromCache(TokenNameCameraVideo)
		assert.NotNil(t, token)
	})

	t.Run("cache not is token val", func(t *testing.T) {
		t.Parallel()
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		b := box.(*baseBox)
		b.tokenCache.Store(TokenNameCameraVideo, "abc")
		token := b.getTokenFromCache(TokenNameCameraVideo)
		assert.Nil(t, token)
	})
}

func TestGetTokenByBox(t *testing.T) {
	t.Run("client is nil", func(t *testing.T) {
		t.Parallel()
		cfg := configs.NewEmptyConfig()
		b := New(&cfg, configs.BoxInfo{}, nil)

		token, err := b.GetTokenByBox("any")
		assert.Nil(t, token)
		assert.Equal(t, err, ErrNoAPIClient)
	})

	t.Run("client is not nil", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		b := box.(*baseBox)
		client := mock.NewMockClient(ctrl)
		b.apiClient = client

		tk := &cloud.Token{}
		client.EXPECT().GetTokenByBox(gomock.Eq("test_token")).Return(tk, nil)
		token, err := b.GetTokenByBox("test_token")

		if assert.Nil(t, err) {
			assert.Equal(t, tk, token)
		}
	})

	t.Run("concurrent get", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cfg := configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		b := box.(*baseBox)
		client := mock.NewMockClient(ctrl)
		b.apiClient = client

		tk := &cloud.Token{}
		client.EXPECT().GetTokenByBox(gomock.Eq("test_token")).Return(tk, nil).AnyTimes()
		go func() {
			token, err := b.GetTokenByBox("test_token")

			if assert.Nil(t, err) {
				assert.Equal(t, tk, token)
			}
		}()
		token, err := b.GetTokenByBox("test_token")

		if assert.Nil(t, err) {
			assert.Equal(t, tk, token)
		}
	})
}

func TestGetTokenByCamera(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	var cfg configs.BaseConfig
	var b *baseBox
	init := func() {
		cfg = configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		b = box.(*baseBox)
		b.tokenCache = &sync.Map{}
	}
	t.Run("get token from cache", func(t *testing.T) {
		init()
		cameraId := 10
		cacheKey := fmt.Sprintf("%s_%d_%d", TokenNameCameraEvent, cameraId, cloud.GetEventExpireDayByCameraID(cameraId))
		b.setTokenInCache(cacheKey, cloud.Token{Url: "url"})
		token, err := b.GetTokenByCamera(cameraId, TokenNameCameraEvent)
		assert.Nil(t, err)
		assert.Equal(t, &cloud.Token{Url: "url"}, token)
	})

	t.Run("no cache", func(t *testing.T) {
		init()
		b.apiClient = nil
		cameraId := 10
		token, err := b.GetTokenByCamera(cameraId, TokenNameCameraEvent)
		assert.NotNil(t, err)
		assert.Nil(t, token)
	})

	t.Run("api not nil", func(t *testing.T) {
		t.Parallel()
		init()
		cameraId := 10
		client := mock.NewMockClient(ctrl)
		b.apiClient = client

		tk := &cloud.Token{Url: "cam_url"}
		client.EXPECT().GetTokenByCamera(cameraId, TokenNameCameraEvent).Return(tk, nil).AnyTimes()
		go func() {
			token, err := b.GetTokenByCamera(cameraId, TokenNameCameraEvent)

			if assert.Nil(t, err) {
				assert.Equal(t, tk, token)
			}
		}()
		token, err := b.GetTokenByCamera(cameraId, TokenNameCameraEvent)

		if assert.Nil(t, err) {
			assert.Equal(t, tk, token)
		}
	})
}

func TestTokenCacheKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	var cfg configs.BaseConfig
	var b *baseBox
	init := func() {
		cfg = configs.NewEmptyConfig()
		box := New(&cfg, configs.BoxInfo{}, nil)
		b = box.(*baseBox)
		b.tokenCache = &sync.Map{}
		cloud.ClearCameraSettings()
	}
	t.Run("camera setting changed", func(t *testing.T) {
		init()
		cameraId := 10
		cacheKey := fmt.Sprintf("%s_%d_%d", TokenNameCameraEvent, cameraId, cloud.GetEventExpireDayByCameraID(cameraId))
		b.setTokenInCache(cacheKey, cloud.Token{Url: "url"})
		token, err := b.GetTokenByCamera(cameraId, TokenNameCameraEvent)

		assert.Nil(t, err)
		assert.Equal(t, &cloud.Token{Url: "url"}, token)

		mockSetting := &cloud.CameraSettings{
			CamID:           10,
			CamSN:           "cam_sn",
			CloudEventTypes: []string{}, // api in cloud will ignore this param, so just give []string{}
			CloudEventMeta: &cloud.CloudEventMeta{
				Duration: 90 * 86400,
			},
			StreamSettings:    []uniview.VideoStreamInfo{},
			VideoCapabilities: &uniview.MediaVideoCapabilities{},
			AudioSettings:     []uniview.AudioStatus{},
			OSDSettings:       &uniview.OSDSetting{},
			OSDCapabilities:   &uniview.OSDCapabilities{},
		}
		cloud.SaveCameraSettings([]*cloud.CameraSettings{mockSetting})
		client := mock.NewMockClient(ctrl)
		b.apiClient = client

		tk := &cloud.Token{Url: "new_url"}
		client.EXPECT().GetTokenByCamera(10, TokenNameCameraEvent).Return(tk, nil).AnyTimes()

		token, err = b.GetTokenByCamera(cameraId, TokenNameCameraEvent)
		assert.Nil(t, err)
		assert.Equal(t, &cloud.Token{Url: "new_url"}, token)
		_, ok := b.tokenCache.Load(fmt.Sprintf("%s_%d_%d", TokenNameCameraEvent, cameraId, 0))
		assert.True(t, ok)
		_, ok = b.tokenCache.Load(fmt.Sprintf("%s_%d_%d", TokenNameCameraEvent, cameraId, cloud.GetEventExpireDayByCameraID(cameraId)))
		assert.True(t, ok)
	})
}
