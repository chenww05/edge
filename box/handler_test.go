package box

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	univiewapi "github.com/example/goshawk/uniview"
	"github.com/example/onvif"
	"github.com/example/turing-common/websocket"

	"github.com/example/minibox/camera"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/configs"
	"github.com/example/minibox/discover"
	"github.com/example/minibox/discover/arp"
	"github.com/example/minibox/mock"
	"github.com/example/minibox/scheduler"
	"github.com/example/minibox/utils"
)

type message struct {
	ErrInfo websocket.Err `json:"err"`
}

func isErrorReply(data []byte) bool {
	var req message
	if err := json.Unmarshal(data, &req); err != nil {
		return false
	}

	return req.ErrInfo.Code == -1
}

func TestRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	called := false
	device := mock.NewMockBox(ctrl)
	logger := zerolog.New(zerolog.NewConsoleWriter())
	h := &handler{log: logger, device: device}
	h.registeredActions = map[string]func(websocket.Message) ([]byte, error){
		"test": func(websocket.Message) ([]byte, error) {
			called = true
			return nil, nil
		},
	}

	req, _ := json.Marshal(map[string]interface{}{
		"act": "test",
	})

	data, err := h.Handle(req)
	assert.True(t, called)
	assert.NoError(t, err)
	assert.Nil(t, data)
}

func TestDefaultAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	device := mock.NewMockBox(ctrl)
	logger := zerolog.New(zerolog.NewConsoleWriter())
	h := &handler{log: logger, device: device}

	req, _ := json.Marshal(map[string]interface{}{
		"act": "test",
	})

	data, err := h.Handle(req)
	assert.Error(t, err)
	assert.Nil(t, data)
}

type encoderMatcher struct {
	isH265 bool
}

func (e encoderMatcher) Matches(x interface{}) bool {
	v := reflect.ValueOf(x)
	if v.Kind() != reflect.String {
		return false
	}
	str := v.String()
	if e.isH265 && strings.Contains(str, defaultDataPath) {
		return true
	}
	if !e.isH265 && strings.Contains(str, defaultPlayPath) {
		return true
	}
	return false
}

func (e encoderMatcher) String() string {
	return ""
}

func TestGetSdpRemote(t *testing.T) {
	type req struct {
		srsIp    string
		srsPort  int64
		sdpLocal string
		cameraId int
		streamId string
	}

	h264 := encoderMatcher{
		isH265: false,
	}
	h265 := encoderMatcher{
		isH265: true,
	}

	var tableSdp = []struct {
		name      string
		req       req
		sdpRemote string
	}{
		{
			name: "264",
			req: req{
				srsIp:    "192.168.1.123",
				srsPort:  1985,
				sdpLocal: "sdp_of_h264",
				cameraId: 123,
				streamId: "123456",
			},
			sdpRemote: "sdp_remote_of_h264",
		},
		{
			name: "265",
			req: req{
				srsIp:    "192.168.1.123",
				srsPort:  1985,
				sdpLocal: "sdp_of_h265(webrtc-datachannel)",
				cameraId: 123,
				streamId: "123456",
			},
			sdpRemote: "sdp_remote_of_h265",
		},
	}

	h264Remote := srsApiRsp{
		Code:      0,
		Server:    "192.168.1.123",
		Sdp:       "sdp_remote_of_h264",
		Sessionid: "sessionid",
	}
	h265Remote := srsApiRsp{
		Code:      0,
		Server:    "192.168.1.123",
		Sdp:       "sdp_remote_of_h265",
		Sessionid: "sessionid",
	}
	msgBytesH264, _ := json.Marshal(h264Remote)
	msgBytesH265, _ := json.Marshal(h265Remote)

	for _, p := range tableSdp {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			srs := mock.NewMockSrs(ctrl)
			if p.name == "264" {
				srs.EXPECT().GetSdpRemote(h264, gomock.Any()).Return(msgBytesH264, nil).Times(1)
			} else if p.name == "265" {
				srs.EXPECT().GetSdpRemote(h265, gomock.Any()).Return(msgBytesH265, nil).Times(1)
			}
			b := baseBox{
				srs: srs,
			}
			sdp_remote, err := b.GetSdpRemote(p.req.srsIp, p.req.srsPort, p.req.sdpLocal, p.req.cameraId, p.req.streamId)
			assert.Equal(t, p.sdpRemote, sdp_remote)
			assert.Nil(t, err)
		})
	}
}

func TestSdpTransport(t *testing.T) {
	var tables_error_param = []struct {
		Name     string
		Params   map[string]interface{}
		Expected bool
	}{
		{
			"lack sdp",
			map[string]interface{}{
				"act": SdpTransport,
				"arg": map[string]interface{}{
					"camera_id": "123",
					"stream_id": "456",
				},
			},
			true,
		},
		{
			"lack camera_id",
			map[string]interface{}{
				"act": SdpTransport,
				"arg": map[string]interface{}{
					"stream_id":  "456",
					"sdp_remote": "sdp_remote info",
				},
			},
			true,
		},
		{
			"lack stream_id",
			map[string]interface{}{
				"act": SdpTransport,
				"arg": map[string]interface{}{
					"camera_id":  "123",
					"sdp_remote": "sdp_remote info",
				},
			},
			true,
		},
	}
	for _, p := range tables_error_param {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			config := mock.NewMockConfig(ctrl)
			device := mock.NewMockBox(ctrl)
			device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
			device.EXPECT().GetConfig().Return(config).AnyTimes()
			config.EXPECT().GetStreamConfig().Return(configs.StreamConfig{}).AnyTimes()
			device.EXPECT().GetSrsIp().Return("enp2s0", nil).AnyTimes()
			device.EXPECT().GetSdpRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("sdp_remote", nil).AnyTimes()
			h := NewHandler(device)
			req, _ := json.Marshal(p.Params)
			_, err := h.Handle(req)
			assert.Error(t, err)
		})
	}

	var tables_ok = []struct {
		Name     string
		Params   map[string]interface{}
		Expected bool
	}{
		{
			"lack sdp",
			map[string]interface{}{
				"id":  "1",
				"act": SdpTransport,
				"arg": map[string]interface{}{
					"camera_id": 1,
					"stream_id": "123456",
					"sdp_local": "sdp_local",
				},
			},
			false,
		},
	}
	for _, p := range tables_ok {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			config := mock.NewMockConfig(ctrl)
			device := mock.NewMockBox(ctrl)
			device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
			device.EXPECT().GetConfig().Return(config).AnyTimes()
			config.EXPECT().GetStreamConfig().Return(configs.StreamConfig{}).AnyTimes()
			device.EXPECT().GetSrsIp().Return("enp2s0", nil).AnyTimes()
			device.EXPECT().GetSdpRemote(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("sdp_remote", nil).AnyTimes()
			h := NewHandler(device)
			req, _ := json.Marshal(p.Params)
			_, err := h.Handle(req)
			assert.Nil(t, err)
		})
	}
}

func TestStartMultiStream(t *testing.T) {
	scheduler.InitScheduler(configs.StreamConfig{
		MaxWaitingQueueSize: 100,
		MaxLivestreamSize:   17,
	})

	var tables = []struct {
		Name     string
		Params   map[string]interface{}
		Expected bool
	}{
		{
			"no args",
			map[string]interface{}{
				"act": StartMultiStream,
			},
			true,
		},
		{
			"wrong id type",
			map[string]interface{}{
				"act": StartMultiStream,
				"arg": map[string]interface{}{
					"param": []map[string]interface{}{{"camera_id": 1}},
				},
			},
			true,
		},
	}
	for _, p := range tables {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			device := mock.NewMockBox(ctrl)
			device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
			h := NewHandler(device)
			req, _ := json.Marshal(p.Params)
			data, err := h.Handle(req)
			assert.Error(t, err)
			assert.Equal(t, p.Expected, isErrorReply(data))
		})
	}

	tables2 := []struct {
		Name     string
		Params   map[string]interface{}
		Expected bool
	}{
		{
			"not found camera",
			map[string]interface{}{
				"act": StartMultiStream,
				"arg": map[string]interface{}{
					"param": []map[string]interface{}{{"camera_id": 1}},
				},
			},
			true,
		},
	}

	for _, p := range tables2 {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			device := mock.NewMockBox(ctrl)
			device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
			h := NewHandler(device)

			req, _ := json.Marshal(p.Params)
			data, err := h.Handle(req)
			assert.Error(t, err)
			assert.Equal(t, p.Expected, isErrorReply(data))
		})
	}

	tables3 := []struct {
		Name     string
		Params   map[string]interface{}
		Expected bool
	}{
		{
			"without token",
			map[string]interface{}{
				"act": StartMultiStream,
				"arg": map[string]interface{}{
					"param": []map[string]interface{}{{"camera_id": 1}},
				},
			},
			true,
		},
		{
			"without uri",
			map[string]interface{}{
				"act": StartStream,
				"arg": map[string]interface{}{
					"camera_id": 1,
					"param":     []map[string]interface{}{{"camera_id": 1, "token": map[string]interface{}{}}},
				},
			},
			true,
		},
		{
			"with uri but no local camera uri",
			map[string]interface{}{
				"act": StartMultiStream,
				"arg": map[string]interface{}{
					"camera_id": 1,
					"param": []map[string]interface{}{
						{"camera_id": 1, "token": map[string]interface{}{
							"uri":      "rtsps://localhost:8554/orgs/77/users/208/cameras/373",
							"base_uri": "rtsps://dev-stream.example.cn:8554",
						}},
					},
				},
			},
			true,
		},
	}

	for _, p := range tables3 {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			device := mock.NewMockBox(ctrl)
			device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
			h := NewHandler(device)

			req, _ := json.Marshal(p.Params)
			data, err := h.Handle(req)
			assert.Error(t, err)
			assert.Equal(t, p.Expected, isErrorReply(data))
		})
	}

	// right test， only test in local develop environment with installed ffmpeg, camera, easydarwin
	tables4 := []struct {
		Name     string
		Params   map[string]interface{}
		Expected bool
	}{
		{
			"right params",
			map[string]interface{}{
				"id":  "example",
				"act": StartMultiStream,
				"arg": map[string]interface{}{
					"camera_id": 1,
					"param": []map[string]interface{}{
						{"camera_id": 1, "token": map[string]interface{}{
							"uri":      "rtsps://localhost:8554/orgs/77/users/208/cameras/373",
							"base_uri": "rtsps://dev-stream.example.cn:8554",
						}},
					},
				},
			},
			false,
		},
		{
			"right params",
			map[string]interface{}{
				"id":  "example",
				"act": StartMultiStream,
				"arg": map[string]interface{}{
					"camera_id": 1,
					"param": []map[string]interface{}{
						{"camera_id": 1,
							"token": map[string]interface{}{
								"uri":      "rtsps://localhost:8554/orgs/77/users/208/cameras/373",
								"base_uri": "rtsps://dev-stream.example.cn:8554",
							},
							"stream_id": "stream_id"},
					},
				},
			},
			false,
		},
	}

	for _, p := range tables4 {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			config := mock.NewMockConfig(ctrl)
			device := mock.NewMockBox(ctrl)
			device.EXPECT().GetConfig().Return(config).AnyTimes()
			device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
			nc := univiewapi.NewMockClient(ctrl)
			nc.EXPECT().GetChannelStreamDetailInfo(uint32(0)).Return(&univiewapi.VideoStreamInfos{}, nil).AnyTimes()
			nvrManager := mock.NewMockNVRManager(ctrl)
			nvrManager.EXPECT().GetNVRClientBySN("").Return(nc, nil).AnyTimes()
			device.EXPECT().GetNVRManager().Return(nvrManager).AnyTimes()

			config.EXPECT().GetStreamConfig().Return(configs.StreamConfig{}).AnyTimes()
			h := NewHandler(device)

			req, _ := json.Marshal(p.Params)
			// c := mock.NewMockCamera(ctrl)
			config.EXPECT().GetDataStoreDir().Return("./data")
			device.EXPECT().GetCamera(gomock.Eq(1)).Return(camera.NewCamera(&cloud.Camera{
				ID: 1, SN: "test_sn1", Brand: string(utils.Sunell), Uri: "rtsp://admin:admin123@192.168.2.250:554/snl/live/1/1", HdUri: "rtsp://admin:admin123@192.168.2.250:554/snl/live/1/1"},
				utils.Sunell, config), nil)

			data, err := h.Handle(req)

			assert.Nil(t, err)
			assert.Equal(t, p.Expected, isErrorReply(data))
		})
	}
}

func TestStartStream(t *testing.T) {
	scheduler.InitScheduler(configs.StreamConfig{
		MaxWaitingQueueSize: 100,
		MaxLivestreamSize:   17,
	})
	var tables = []struct {
		Name     string
		Params   map[string]interface{}
		Expected bool
	}{
		{
			"no args",
			map[string]interface{}{
				"act": StartStream,
			},
			true,
		},
		{
			"wrong id type",
			map[string]interface{}{
				"act": StartStream,
				"arg": map[string]interface{}{
					"camera_id": "1",
				},
			},
			true,
		},
	}
	for _, p := range tables {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			device := mock.NewMockBox(ctrl)
			device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
			h := NewHandler(device)
			req, _ := json.Marshal(p.Params)
			data, err := h.Handle(req)
			assert.Error(t, err)
			assert.Equal(t, p.Expected, isErrorReply(data))
		})
	}

	tables2 := []struct {
		Name     string
		Params   map[string]interface{}
		Expected bool
	}{
		{
			"not found camera",
			map[string]interface{}{
				"act": StartStream,
				"arg": map[string]interface{}{
					"camera_id": 1,
				},
			},
			true,
		},
	}

	for _, p := range tables2 {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			device := mock.NewMockBox(ctrl)
			device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
			h := NewHandler(device)

			req, _ := json.Marshal(p.Params)
			data, err := h.Handle(req)
			assert.Error(t, err)
			assert.Equal(t, p.Expected, isErrorReply(data))
		})
	}

	tables3 := []struct {
		Name     string
		Params   map[string]interface{}
		Expected bool
	}{
		{
			"without token",
			map[string]interface{}{
				"act": StartStream,
				"arg": map[string]interface{}{
					"camera_id": 1,
				},
			},
			true,
		},
		{
			"without uri",
			map[string]interface{}{
				"act": StartStream,
				"arg": map[string]interface{}{
					"camera_id": 1,
					"token":     map[string]interface{}{},
				},
			},
			true,
		},
		{
			"with uri but no local camera uri",
			map[string]interface{}{
				"act": StartStream,
				"arg": map[string]interface{}{
					"camera_id": 1,
					"token": map[string]interface{}{
						"uri":      "rtsps://localhost:8554/orgs/77/users/208/cameras/373",
						"base_uri": "rtsps://dev-stream.example.cn:8554",
					},
				},
			},
			true,
		},
	}

	for _, p := range tables3 {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			device := mock.NewMockBox(ctrl)
			device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
			h := NewHandler(device)

			req, _ := json.Marshal(p.Params)
			data, err := h.Handle(req)
			assert.Error(t, err)
			assert.Equal(t, p.Expected, isErrorReply(data))
		})
	}

	// right test， only test in local develop environment with installed ffmpeg, camera, easydarwin
	tables4 := []struct {
		Name     string
		Params   map[string]interface{}
		Expected bool
	}{
		{
			"right params",
			map[string]interface{}{
				"id":  "example",
				"act": StartStream,
				"arg": map[string]interface{}{
					"camera_id": 1,
					"token": map[string]interface{}{
						"uri":      "rtsp://localhost:8554/orgs/77/users/208/cameras/373",
						"base_uri": "rtsp://localhost:8554",
					},
				},
			},
			false,
		},
		{
			"right params",
			map[string]interface{}{
				"id":  "example",
				"act": StartStream,
				"arg": map[string]interface{}{
					"camera_id": 1,
					"token": map[string]interface{}{
						"uri":      "rtsp://localhost:8554/orgs/77/users/208/cameras/373",
						"base_uri": "rtsp://localhost:8554",
					},
					"stream_param": map[string]interface{}{
						"stream_id": "stream id",
					},
				},
			},
			false,
		},
	}

	for _, p := range tables4 {
		t.Run(p.Name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			config := mock.NewMockConfig(ctrl)
			device := mock.NewMockBox(ctrl)
			device.EXPECT().GetConfig().Return(config).AnyTimes()
			device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
			nc := univiewapi.NewMockClient(ctrl)
			nc.EXPECT().GetChannelStreamDetailInfo(uint32(0)).Return(&univiewapi.VideoStreamInfos{}, nil).AnyTimes()
			nvrManager := mock.NewMockNVRManager(ctrl)
			nvrManager.EXPECT().GetNVRClientBySN("").Return(nc, nil).AnyTimes()
			device.EXPECT().GetNVRManager().Return(nvrManager).AnyTimes()

			config.EXPECT().GetStreamConfig().Return(configs.StreamConfig{}).AnyTimes()
			h := NewHandler(device)

			req, _ := json.Marshal(p.Params)
			// c := mock.NewMockCamera(ctrl)
			config.EXPECT().GetDataStoreDir().Return("./data")
			device.EXPECT().GetCamera(gomock.Eq(1)).Return(camera.NewCamera(&cloud.Camera{
				ID: 1, SN: "test_sn1", Brand: string(utils.Sunell), Uri: "rtsp://admin:admin123@192.168.2.250:554/snl/live/1/1", HdUri: "rtsp://admin:admin123@192.168.2.250:554/snl/live/1/1"},
				utils.Sunell, config), nil)

			data, err := h.Handle(req)

			assert.Nil(t, err)
			assert.Equal(t, p.Expected, isErrorReply(data))
		})
	}
}

func TestGetUpload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	device := mock.NewMockBox(ctrl)
	device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
	h := NewHandler(device)

	req, _ := json.Marshal(map[string]interface{}{
		"act": "box.get_upload_config",
	})

	cfg := mock.NewMockConfig(ctrl)
	device.EXPECT().GetConfig().Return(cfg)
	cfg.EXPECT().GetUploadConfig().Return(&configs.UploadConfig{})

	_, err := h.Handle(req)
	assert.NoError(t, err)
}

func TestSetUpload(t *testing.T) {
	t.Run("no args - error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		device := mock.NewMockBox(ctrl)
		device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		h := NewHandler(device)

		req, _ := json.Marshal(map[string]interface{}{
			"act": "box.set_upload_config",
		})

		cfg := configs.NewEmptyConfig()

		device.EXPECT().GetConfig().Return(&cfg)
		device.EXPECT().SetUploadConfig(gomock.Any()).Return(errors.New("can't set config"))

		data, err := h.Handle(req)
		assert.Error(t, err)
		assert.True(t, isErrorReply(data))
	})

	t.Run("no args - empty", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		device := mock.NewMockBox(ctrl)
		device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		h := NewHandler(device)

		req, _ := json.Marshal(map[string]interface{}{
			"act": "box.set_upload_config",
		})

		cfg := configs.NewEmptyConfig()

		device.EXPECT().GetConfig().Return(&cfg)
		device.EXPECT().SetUploadConfig(gomock.Eq(configs.UploadConfig{})).Return(nil)

		_, err := h.Handle(req)
		assert.NoError(t, err)
	})

	t.Run("with args", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		device := mock.NewMockBox(ctrl)
		device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		h := NewHandler(device)

		req, _ := json.Marshal(map[string]interface{}{
			"act": "box.set_upload_config",
			"arg": map[string]interface{}{
				"disable_upload_picture":     true,
				"disable_upload_temperature": true,
				"disable_cloud":              true,
				"camera_scanperiod_secs":     20,
			},
		})

		cfg := configs.NewEmptyConfig()

		device.EXPECT().GetConfig().Return(&cfg)
		device.EXPECT().SetUploadConfig(gomock.Eq(configs.UploadConfig{
			DisableCloud:             true,
			DisableUploadTemperature: true,
			DisableUploadPic:         true,
			CameraScanPeriod:         20,
		})).Return(nil)

		_, err := h.Handle(req)
		assert.NoError(t, err)
	})

	t.Run("bad upload_picture", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		device := mock.NewMockBox(ctrl)
		device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		h := NewHandler(device)

		req, _ := json.Marshal(map[string]interface{}{
			"act": "box.set_upload_config",
			"arg": map[string]interface{}{
				"disable_upload_picture": "some_val",
			},
		})

		cfg := configs.NewEmptyConfig()

		device.EXPECT().GetConfig().Return(&cfg)

		data, err := h.Handle(req)
		if assert.Error(t, err) {
			assert.Equal(t, ErrArgsWrongType, err)
		}
		assert.True(t, isErrorReply(data))
	})

	t.Run("bad upload_temp", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		device := mock.NewMockBox(ctrl)
		device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		h := NewHandler(device)

		req, _ := json.Marshal(map[string]interface{}{
			"act": "box.set_upload_config",
			"arg": map[string]interface{}{
				"disable_upload_temperature": "some_val",
			},
		})

		cfg := configs.NewEmptyConfig()

		device.EXPECT().GetConfig().Return(&cfg)

		data, err := h.Handle(req)
		if assert.Error(t, err) {
			assert.Equal(t, ErrArgsWrongType, err)
		}
		assert.True(t, isErrorReply(data))
	})

	t.Run("bad cloud", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		device := mock.NewMockBox(ctrl)
		device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		h := NewHandler(device)

		req, _ := json.Marshal(map[string]interface{}{
			"act": "box.set_upload_config",
			"arg": map[string]interface{}{
				"disable_cloud": "some_val",
			},
		})

		cfg := configs.NewEmptyConfig()

		device.EXPECT().GetConfig().Return(&cfg)

		data, err := h.Handle(req)
		if assert.Error(t, err) {
			assert.Equal(t, ErrArgsWrongType, err)
		}
		assert.True(t, isErrorReply(data))
	})

	t.Run("bad scan period", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		device := mock.NewMockBox(ctrl)
		device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		h := NewHandler(device)

		req, _ := json.Marshal(map[string]interface{}{
			"act": "box.set_upload_config",
			"arg": map[string]interface{}{
				"camera_scanperiod_secs": "some_val",
			},
		})

		cfg := configs.NewEmptyConfig()

		device.EXPECT().GetConfig().Return(&cfg)

		data, err := h.Handle(req)
		if assert.Error(t, err) {
			assert.Equal(t, ErrArgsWrongType, err)
		}
		assert.True(t, isErrorReply(data))
	})
}

func TestSetBoxTimeZone(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	device := mock.NewMockBox(ctrl)
	device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
	h := NewHandler(device)

	req, _ := json.Marshal(map[string]interface{}{
		"act": SetTimeZone,
		"arg": map[string]interface{}{
			"box_id":   "box_mini_10014",
			"timezone": "Asia/Shanghai",
		},
	})

	c := mock.NewMockConfig(ctrl)
	device.EXPECT().GetConfig().Return(c).Times(1)
	c.EXPECT().SetTimeZone(gomock.Any()).Times(1)
	nvrManager := mock.NewMockNVRManager(ctrl)
	device.EXPECT().GetNVRManager().Return(nvrManager).Times(1)
	nvrManager.EXPECT().SetTimeZone(gomock.Any()).Times(1)
	data, err := h.Handle(req)

	assert.NoError(t, err)
	assert.Nil(t, data)
}

func Test_handler_handleBoxSetting(t *testing.T) {
	type fields struct {
		log               zerolog.Logger
		registeredActions map[string]func(websocket.Message) ([]byte, error)
		device            Box
		searcher          *discover.SearcherProcessor
	}
	type args struct {
		msg websocket.Message
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	var device = mock.NewMockBox(ctrl)
	searcher := discover.NewSearcherProcessor()
	c := mock.NewMockConfig(ctrl)
	device.EXPECT().GetConfig().Return(c).AnyTimes()
	scheduler.InitScheduler(configs.StreamConfig{})

	msgGenerator := func(livestreamSize, maxPlaybackSize, eventSavedHours, eventMaxRetry int) websocket.Message {
		req, _ := json.Marshal(map[string]interface{}{
			"act": UpdateBoxSetting,
			"arg": map[string]interface{}{
				"max_livestream_size": livestreamSize,
				"max_playback_size":   maxPlaybackSize,
				"event_saved_hours":   eventSavedHours,
				"event_max_retry":     eventMaxRetry,
			},
		})
		msg, _ := websocket.ToMessage(req)
		return msg
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		prepare func()
		want    map[string]int
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "all size 0",
			args: args{
				msg: msgGenerator(0, 0, 0, 0),
			},
			prepare: func() {
				c.EXPECT().GetStreamConfig().Return(configs.StreamConfig{
					MaxClipSize:       0,
					MaxLivestreamSize: 0,
				}).AnyTimes()
				c.EXPECT().GetEventRetryCount().Return(uint(0)).AnyTimes()
				c.EXPECT().GetEventSavedHours().Return(0).AnyTimes()
			},
			want: map[string]int{"max_playback_size": 0, "max_livestream_size": 0, "event_saved_hours": 0, "event_max_retry": 0},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return err == nil
			},
		},
		{
			name: "all size 1",
			args: args{
				msg: msgGenerator(1, 1, 1, 1),
			},
			prepare: func() {
				c.EXPECT().SetMaxClipSize(1)
				c.EXPECT().SetMaxLivestreamSize(1)
				c.EXPECT().SetEventSavedHours(1)
				c.EXPECT().SetEventRetryCount(1)
				c.EXPECT().GetStreamConfig().Return(configs.StreamConfig{
					MaxLivestreamSize: 1,
					MaxClipSize:       1,
				}).AnyTimes()
				c.EXPECT().GetEventRetryCount().Return(uint(1)).AnyTimes()
				c.EXPECT().GetEventSavedHours().Return(1).AnyTimes()
			},
			want: map[string]int{"max_playback_size": 1, "max_livestream_size": 1, "event_saved_hours": 1, "event_max_retry": 1},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return err == nil
			},
		},
		{
			name: "params are different",
			args: args{
				msg: msgGenerator(2, 1, 3, 4),
			},
			prepare: func() {
				c.EXPECT().SetMaxClipSize(1)
				c.EXPECT().SetMaxLivestreamSize(2)
				c.EXPECT().SetEventSavedHours(3)
				c.EXPECT().SetEventRetryCount(4)
				c.EXPECT().GetStreamConfig().Return(configs.StreamConfig{
					MaxClipSize:       1,
					MaxLivestreamSize: 2,
				}).AnyTimes()
				c.EXPECT().GetEventRetryCount().Return(uint(4)).AnyTimes()
				c.EXPECT().GetEventSavedHours().Return(3).AnyTimes()
			},
			want: map[string]int{"max_playback_size": 1, "max_livestream_size": 2, "event_saved_hours": 3, "event_max_retry": 4},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return err == nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := &handler{
				log:      tt.fields.log,
				device:   device,
				searcher: searcher,
			}
			tt.prepare()
			got, err := h.handleBoxSetting(tt.args.msg)
			if !tt.wantErr(t, err, fmt.Sprintf("handleBoxSetting(%v)", tt.args.msg)) {
				return
			}
			msg, err := websocket.ToMessage(got)
			data := msg.GetReturn()
			var bsr UpdateBoxSettingReq
			mapstructure.Decode(data, &bsr)

			assert.Equalf(t, tt.want["max_livestream_size"], bsr.MaxLivestreamSize, "handleBoxSetting(%v)", tt.args.msg)
			assert.Equalf(t, tt.want["max_playback_size"], bsr.MaxPlaybackSize, "handleBoxSetting(%v)", tt.args.msg)
			assert.Equalf(t, tt.want["event_saved_hours"], bsr.EventSavedHours, "handleBoxSetting(%v)", tt.args.msg)
			assert.Equalf(t, tt.want["event_max_retry"], bsr.EventMaxRetry, "handleBoxSetting(%v)", tt.args.msg)

		})
	}
}

func Test_updateCameraFromCloud(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var h websocket.Handler
	var box *mock.MockBox
	init := func() {
		box = mock.NewMockBox(ctrl)
		box.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		h = NewHandler(box)
	}

	t.Run("not ai cam", func(t *testing.T) {
		init()
		msg, _ := json.Marshal(map[string]interface{}{
			"act": UpdateCamera,
			"arg": map[string]interface{}{
				"camera_id":            10,
				"name":                 "camera_10",
				"brand":                "brand",
				"uri":                  "rtsp://192.168.11.128/unicast/c1/s1/live",
				"hd_uri":               "rtsp://192.168.11.128/unicast/c1/s0/live",
				"sd_uri":               "rtsp://192.168.11.128/unicast/c1/s2/live",
				"nvr_sn":               "nvr_sn",
				"username":             "username",
				"password":             "password",
				"upload_video_enabled": true,
				"detect_params":        map[string]interface{}{},
			},
		})
		cam := mock.NewMockThermal1Camera(ctrl)
		box.EXPECT().GetCamera(10).Return(cam, nil).Times(1)
		cam.EXPECT().UpdateCameraFromCloud(gomock.Any()).Times(1)
		_, err := h.Handle(msg)
		assert.Nil(t, err)
	})

	t.Run("ai cam not plugged", func(t *testing.T) {
		init()
		msg, _ := json.Marshal(map[string]interface{}{
			"act": UpdateCamera,
			"arg": map[string]interface{}{
				"camera_id":            10,
				"name":                 "camera_10",
				"brand":                "brand",
				"uri":                  "rtsp://192.168.11.128/unicast/c1/s1/live",
				"hd_uri":               "rtsp://192.168.11.128/unicast/c1/s0/live",
				"sd_uri":               "rtsp://192.168.11.128/unicast/c1/s2/live",
				"nvr_sn":               "nvr_sn",
				"username":             "username",
				"password":             "password",
				"upload_video_enabled": true,
				"detect_params":        map[string]interface{}{},
			},
		})
		cam := mock.NewMockAICamera(ctrl)
		box.EXPECT().GetCamera(10).Return(cam, nil).Times(1)
		nvrMock := mock.NewMockNVRManager(ctrl)
		box.EXPECT().GetNVRManager().Return(nvrMock).AnyTimes()
		cam.EXPECT().GetOnline().Return(true).AnyTimes()
		cam.EXPECT().GetID().Return(10).AnyTimes()
		nvrMock.EXPECT().IsCameraPlugged(cam).Return(false).Times(1)

		cloudApi := mock.NewMockClient(ctrl)
		box.EXPECT().CloudClient().Return(cloudApi).AnyTimes()
		cloudApi.EXPECT().GetNvrs().Return([]*cloud.NVR{}, nil).Times(1)
		box.EXPECT().SetCloudNvrs([]*cloud.NVR{}).Return().Times(1)

		nvrMock.EXPECT().UpdateNvr().Times(1)
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nc := univiewapi.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nc, nil)
		cam.EXPECT().SetNVRClient(nc).Return(nil).Times(1)
		_, err := h.Handle(msg)
		assert.NotNil(t, err)
	})

	t.Run("ai cam offline", func(t *testing.T) {
		init()
		msg, _ := json.Marshal(map[string]interface{}{
			"act": UpdateCamera,
			"arg": map[string]interface{}{
				"camera_id":            10,
				"name":                 "camera_10",
				"brand":                "brand",
				"uri":                  "rtsp://192.168.11.128/unicast/c1/s1/live",
				"hd_uri":               "rtsp://192.168.11.128/unicast/c1/s0/live",
				"sd_uri":               "rtsp://192.168.11.128/unicast/c1/s2/live",
				"nvr_sn":               "nvr_sn",
				"username":             "username",
				"password":             "password",
				"upload_video_enabled": true,
				"detect_params":        map[string]interface{}{},
			},
		})
		cam := mock.NewMockAICamera(ctrl)
		box.EXPECT().GetCamera(10).Return(cam, nil).Times(1)
		nvrMock := mock.NewMockNVRManager(ctrl)
		box.EXPECT().GetNVRManager().Return(nvrMock).AnyTimes()
		cam.EXPECT().GetOnline().Return(false).AnyTimes()
		cam.EXPECT().GetID().Return(10).AnyTimes()
		nvrMock.EXPECT().IsCameraPlugged(cam).Return(true).Times(1)

		cloudApi := mock.NewMockClient(ctrl)
		box.EXPECT().CloudClient().Return(cloudApi).AnyTimes()
		cloudApi.EXPECT().GetNvrs().Return([]*cloud.NVR{}, nil).Times(1)
		box.EXPECT().SetCloudNvrs([]*cloud.NVR{}).Return().Times(1)

		nvrMock.EXPECT().UpdateNvr().Times(1)
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nc := univiewapi.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nc, nil)
		cam.EXPECT().SetNVRClient(nc).Return(nil).Times(1)
		_, err := h.Handle(msg)
		assert.NotNil(t, err)
	})

	t.Run("ai cam ok", func(t *testing.T) {
		init()
		msg, _ := json.Marshal(map[string]interface{}{
			"act": UpdateCamera,
			"arg": map[string]interface{}{
				"camera_id":            10,
				"name":                 "camera_10",
				"brand":                "brand",
				"uri":                  "rtsp://192.168.11.128/unicast/c1/s1/live",
				"hd_uri":               "rtsp://192.168.11.128/unicast/c1/s0/live",
				"sd_uri":               "rtsp://192.168.11.128/unicast/c1/s2/live",
				"nvr_sn":               "nvr_sn",
				"username":             "username",
				"password":             "password",
				"upload_video_enabled": true,
				"detect_params":        map[string]interface{}{},
			},
		})
		cam := mock.NewMockAICamera(ctrl)
		box.EXPECT().GetCamera(10).Return(cam, nil).Times(1)
		nvrMock := mock.NewMockNVRManager(ctrl)
		box.EXPECT().GetNVRManager().Return(nvrMock).AnyTimes()
		cam.EXPECT().GetOnline().Return(true).AnyTimes()
		cam.EXPECT().GetID().Return(10).AnyTimes()
		nvrMock.EXPECT().IsCameraPlugged(cam).Return(true).Times(1)

		cam.EXPECT().UpdateCameraFromCloud(gomock.Any()).Times(1)

		cloudApi := mock.NewMockClient(ctrl)
		box.EXPECT().CloudClient().Return(cloudApi).AnyTimes()
		cloudApi.EXPECT().GetNvrs().Return([]*cloud.NVR{}, nil).Times(1)
		box.EXPECT().SetCloudNvrs([]*cloud.NVR{}).Return().Times(1)

		nvrMock.EXPECT().UpdateNvr().Times(1)
		cam.EXPECT().GetNvrSN().Return("nvr_sn").Times(1)
		nc := univiewapi.NewMockClient(ctrl)
		nvrMock.EXPECT().GetNVRClientBySN("nvr_sn").Return(nc, nil)
		cam.EXPECT().SetNVRClient(nc).Return(nil).Times(1)
		_, err := h.Handle(msg)
		assert.Nil(t, err)
	})
}

func TestIotDiscover(t *testing.T) {
	t.Run("arp disable", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		box := mock.NewMockBox(ctrl)
		cfg := mock.NewMockConfig(ctrl)
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		box.EXPECT().GetArpSearcher().Return(arpSearcher).AnyTimes()
		box.EXPECT().GetConfig().Return(cfg).AnyTimes()
		h := NewHandler(box).(*handler)

		msg, _ := json.Marshal(map[string]interface{}{
			"act": IotDiscover,
			"arg": map[string]interface{}{
				"box_id": "test_box",
			},
		})
		cfg.EXPECT().GetArpEnable().Return(false).Times(1)
		_, err := h.Handle(msg)
		assert.Error(t, err)
	})

	t.Run("arp enable", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		box := mock.NewMockBox(ctrl)
		cfg := mock.NewMockConfig(ctrl)
		arpSearcher := arp.NewMockSearcherProcessor(ctrl)
		box.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		box.EXPECT().GetArpSearcher().Return(arpSearcher).AnyTimes()
		box.EXPECT().GetConfig().Return(cfg).AnyTimes()
		h := NewHandler(box).(*handler)

		cfg.EXPECT().GetArpEnable().Return(true).Times(2)
		cfg.EXPECT().GetArpReportTimes().Return(uint16(1)).AnyTimes()
		cfg.EXPECT().GetArpReportInterval().Return(uint16(1)).AnyTimes()
		arpSearcher.EXPECT().SearchAll().MaxTimes(1)
		arpSearcher.EXPECT().SearchWithIpFilter(uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.2")), []string{"192.168.0.3"}).MaxTimes(1)

		msg, _ := json.Marshal(map[string]interface{}{
			"act": IotDiscover,
			"arg": map[string]interface{}{
				"box_id": "test_box",
			},
		})
		_, err := h.Handle(msg)

		assert.Nil(t, err)

		msg, _ = json.Marshal(map[string]interface{}{
			"act": IotDiscover,
			"arg": map[string]interface{}{
				"box_id": "test_box",
				"filter": map[string]interface{}{
					"ip_address": []string{"192.168.0.3"},
					"ip_range": map[string]interface{}{
						"from": "192.168.0.1",
						"to":   "192.168.0.2",
					},
				},
			},
		})
		_, err = h.Handle(msg)
		assert.Nil(t, err)
	})
}

func TestMergeDevices(t *testing.T) {
	t.Run("ip ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		box := mock.NewMockBox(ctrl)
		box.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		h := NewHandler(box).(*handler)
		arpDeviceMap := make(map[string]arp.Device)
		arpDeviceMap["xx:xx:xx:xx:xx"] = arp.Device{
			Mac: "xx:xx:xx:xx:xx",
			IP:  "192.168.0.1",
		}
		arpDeviceMap["yy:yy:yy:yy:yy"] = arp.Device{
			Mac: "yy:yy:yy:yy:yy",
			IP:  "192.168.0.2",
		}

		onvifDevice1 := onvif.Device{
			Params: onvif.DeviceParams{
				Xaddr: "192.168.0.1",
			},
			Info: onvif.DeviceInfo{
				Manufacturer: "test",
				MACAddress:   "xx:xx:xx:xx:xx",
			},
		}
		expectDevice1 := IotDevice{
			MacAddress:   "xx:xx:xx:xx:xx",
			IpAddress:    "192.168.0.1",
			Manufacturer: "test",
		}
		expectDevice2 := IotDevice{
			MacAddress: "yy:yy:yy:yy:yy",
			IpAddress:  "192.168.0.2",
		}
		ret := h.mergeDevices(arpDeviceMap, map[string]onvif.Device{"xx:xx:xx:xx:xx": onvifDevice1})

		assert.Equal(t, 2, len(ret))
		if ret[0].IpAddress == "192.168.0.1" {
			assert.Equal(t, []IotDevice{expectDevice1, expectDevice2}, ret)
		} else {
			assert.Equal(t, []IotDevice{expectDevice2, expectDevice1}, ret)
		}
	})

	t.Run("ip conflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		box := mock.NewMockBox(ctrl)
		box.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
		h := NewHandler(box).(*handler)
		arpDeviceMap := make(map[string]arp.Device)
		arpDeviceMap["xx:xx:xx:xx:xx"] = arp.Device{
			Mac: "xx:xx:xx:xx:xx",
			IP:  "192.168.0.1",
		}
		arpDeviceMap["yy:yy:yy:yy:yy"] = arp.Device{
			Mac: "yy:yy:yy:yy:yy",
			IP:  "192.168.0.2",
		}

		onvifDevice1 := onvif.Device{
			Params: onvif.DeviceParams{
				Xaddr: "192.168.0.3",
			},
			Info: onvif.DeviceInfo{
				Manufacturer: "test",
				MACAddress:   "xx:xx:xx:xx:xx",
			},
		}
		expectDevice2 := IotDevice{
			MacAddress: "yy:yy:yy:yy:yy",
			IpAddress:  "192.168.0.2",
		}
		ret := h.mergeDevices(arpDeviceMap, map[string]onvif.Device{"xx:xx:xx:xx:xx": onvifDevice1})
		assert.Equal(t, []IotDevice{expectDevice2}, ret)
	})
}

func TestIsIotDeviceAllFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	box.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
	h := NewHandler(box).(*handler)
	t.Run("ip filter", func(t *testing.T) {
		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
		}, 0, 0, []string{"192.168.0.1", "192.168.0.2"}, []string{}))
		assert.True(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, 0, 0, []string{"192.168.0.1", "192.168.0.2"}, []string{}))
	})
	t.Run("full mac filter", func(t *testing.T) {
		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
		}, 0, 0, []string{}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X2"}))
		assert.True(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, 0, 0, []string{}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X2"}))
	})
	t.Run("mac prefix filter", func(t *testing.T) {
		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, 0, 0, []string{}, []string{"XX:XX:XX:XX:XX"}))
		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, 0, 0, []string{}, []string{"XX:XX:XX:XX:XX:XX:XX"}))
	})
	t.Run("ip range filter", func(t *testing.T) {
		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.10")), []string{}, []string{}))
		assert.True(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.2")), []string{}, []string{}))
	})
	t.Run("no filter", func(t *testing.T) {
		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, 0, 0, []string{}, []string{}))
	})
	t.Run("no ip range filter", func(t *testing.T) {
		assert.True(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, 0, 0, []string{"192.168.0.1", "192.168.0.2"}, []string{"XX:XX:XX:XX:XX:X1"}))
		assert.True(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, 0, 0, []string{"192.168.0.1", "192.168.0.2"}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X2"}))
		assert.True(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, 0, 0, []string{"192.168.0.1"}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X2"}))
		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, 0, 0, []string{"192.168.0.1"}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX"}))
		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, 0, 0, []string{"192.168.0.1"}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X3"}))
	})

	t.Run("no ip filter", func(t *testing.T) {
		assert.True(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.2")), []string{}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X2"}))

		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.10")), []string{}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X2"}))

		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.2")), []string{}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X3"}))

		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.2")), []string{}, []string{"XX:XX:XX:XX:XX", "XX:XX:XX:XX:XX:X1"}))
	})

	t.Run("no mac filter", func(t *testing.T) {
		assert.True(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.2")), []string{"192.168.0.1", "192.168.0.2"}, []string{}))

		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.2")), []string{"192.168.0.1", "192.168.0.3"}, []string{}))

		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.3")), []string{"192.168.0.1", "192.168.0.2"}, []string{}))
	})

	t.Run("all filter", func(t *testing.T) {
		assert.True(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.2")), []string{"192.168.0.1", "192.168.0.2"}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X2"}))

		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.3")), []string{"192.168.0.1", "192.168.0.2"}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X2"}))

		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.2")), []string{"192.168.0.1", "192.168.0.3"}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X2"}))

		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.2")), []string{"192.168.0.1", "192.168.0.2"}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX:X3"}))

		assert.False(t, h.isIotDeviceAllFound([]IotDevice{
			{MacAddress: "XX:XX:XX:XX:XX:X1", IpAddress: "192.168.0.1"},
			{MacAddress: "XX:XX:XX:XX:XX:X2", IpAddress: "192.168.0.2"},
		}, uint32(utils.ParseIPString("192.168.0.1")), uint32(utils.ParseIPString("192.168.0.2")), []string{"192.168.0.1", "192.168.0.2"}, []string{"XX:XX:XX:XX:XX:X1", "XX:XX:XX:XX:XX"}))
	})
}

func Test_handler_getBackwardSpeakerAudioUrl(t *testing.T) {
	type fields struct {
		log               zerolog.Logger
		registeredActions map[string]func(websocket.Message) ([]byte, error)
		device            Box
		searcher          *discover.SearcherProcessor
	}
	type args struct {
		host     string
		username string
		password string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
		{name: "test normal", fields: fields{}, args: args{
			host:     "127.0.0.1/axis/abcd/transmit.cgi",
			username: "root",
			password: "password",
		}, want: "http://root:password@127.0.0.1/axis/abcd/transmit.cgi", wantErr: func(t assert.TestingT, err2 error, i ...interface{}) bool {
			return assert.NoError(t, err2)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler{
				log:               tt.fields.log,
				registeredActions: tt.fields.registeredActions,
				device:            tt.fields.device,
				searcher:          tt.fields.searcher,
			}
			got, err := h.getBackwardSpeakerAudioUrl(tt.args.host, tt.args.username, tt.args.password)
			if !tt.wantErr(t, err, fmt.Sprintf("getBackwardSpeakerAudioUrl(%v, %v, %v)", tt.args.host, tt.args.username, tt.args.password)) {
				return
			}
			assert.Equalf(t, tt.want, got, "getBackwardSpeakerAudioUrl(%v, %v, %v)", tt.args.host, tt.args.username, tt.args.password)
		})
	}
}
