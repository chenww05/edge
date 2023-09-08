package box

import (
	"encoding/json"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	univiewapi "github.com/turingvideo/goshawk/uniview"
	"github.com/turingvideo/minibox/camera"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/discover"
	"github.com/turingvideo/minibox/mock"
	"github.com/turingvideo/minibox/utils"
)

func TestGetPtzPresets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	device := mock.NewMockBox(ctrl)
	config := mock.NewMockConfig(ctrl)
	device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
	device.EXPECT().GetConfig().Return(config).AnyTimes()
	config.EXPECT().GetDataStoreDir().Return("./data")

	nc := univiewapi.NewMockClient(ctrl)
	nvrManager := mock.NewMockNVRManager(ctrl)
	nvrManager.EXPECT().GetNVRClientBySN("").Return(nc, nil).AnyTimes()
	device.EXPECT().GetNVRManager().Return(nvrManager).AnyTimes()
	nc.EXPECT().GetPTZPresets(uint32(0)).Return(&univiewapi.PresetInfoList{}, nil).AnyTimes()

	cam := cloud.Camera{ID: 123, NvrSN: "", Brand: string(utils.Uniview), Channel: 1}
	device.EXPECT().GetCamera(gomock.Eq(123)).Return(camera.NewCamera(&cam, utils.Uniview, config), nil)

	h := NewHandler(device)

	req, _ := json.Marshal(map[string]interface{}{
		"act": "nest.box.camera.get_ptz_presets",
		"arg": map[string]interface{}{
			"camera_id": 123,
		},
	})

	_, err := h.Handle(req)
	assert.NoError(t, err)
}

func TestSetPtzPreset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	device := mock.NewMockBox(ctrl)
	config := mock.NewMockConfig(ctrl)
	device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
	device.EXPECT().GetConfig().Return(config).AnyTimes()
	config.EXPECT().GetDataStoreDir().Return("./data")

	nc := univiewapi.NewMockClient(ctrl)
	nvrManager := mock.NewMockNVRManager(ctrl)
	nvrManager.EXPECT().GetNVRClientBySN("").Return(nc, nil).AnyTimes()
	device.EXPECT().GetNVRManager().Return(nvrManager).AnyTimes()
	nc.EXPECT().PutPTZPreset(uint32(0), uint32(1), &univiewapi.PresetInfo{ID: 1, Name: "Test"}).AnyTimes()

	cam := cloud.Camera{ID: 123, NvrSN: "", Brand: string(utils.Uniview), Channel: 0}
	device.EXPECT().GetCamera(gomock.Eq(123)).Return(camera.NewCamera(&cam, utils.Uniview, config), nil)

	h := NewHandler(device)

	req, _ := json.Marshal(map[string]interface{}{
		"act": "nest.box.camera.set_ptz_preset",
		"arg": map[string]interface{}{
			"camera_id": 123,
			"preset": map[string]interface{}{
				"id":   1,
				"name": "Test",
			},
		},
	})

	_, err := h.Handle(req)
	assert.NoError(t, err)
}

func TestGoToPtzPreset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	device := mock.NewMockBox(ctrl)
	config := mock.NewMockConfig(ctrl)
	device.EXPECT().GetSearcher().Return(discover.NewSearcherProcessor()).AnyTimes()
	device.EXPECT().GetConfig().Return(config).AnyTimes()
	config.EXPECT().GetDataStoreDir().Return("./data")

	nc := univiewapi.NewMockClient(ctrl)
	nvrManager := mock.NewMockNVRManager(ctrl)
	nvrManager.EXPECT().GetNVRClientBySN("").Return(nc, nil).AnyTimes()
	device.EXPECT().GetNVRManager().Return(nvrManager).AnyTimes()
	nc.EXPECT().PutPTZPresetGoTo(uint32(0), uint32(1)).AnyTimes()

	cam := cloud.Camera{ID: 123, NvrSN: "", Brand: string(utils.Uniview), Channel: 0}
	device.EXPECT().GetCamera(gomock.Eq(123)).Return(camera.NewCamera(&cam, utils.Uniview, config), nil)

	h := NewHandler(device)

	req, _ := json.Marshal(map[string]interface{}{
		"act": "nest.box.camera.go_to_ptz_preset",
		"arg": map[string]interface{}{
			"camera_id": 123,
			"preset_id": 1,
		},
	})

	_, err := h.Handle(req)
	assert.NoError(t, err)
}
