package box

import (
	"encoding/json"

	univiewapi "github.com/turingvideo/goshawk/uniview"
	"github.com/turingvideo/minibox/camera/uniview"
	"github.com/turingvideo/minibox/utils"
	"github.com/turingvideo/turing-common/websocket"
)

var ptzCmdMap = map[string]int{
	"turn_left":        univiewapi.PTZTurnLeft,
	"turn_right":       univiewapi.PTZTurnRight,
	"turn_upper":       univiewapi.PTZTurnUpper,
	"turn_lower":       univiewapi.PTZTurnLower,
	"turn_left_upper":  univiewapi.PTZTurnLeftUpper,
	"turn_left_lower":  univiewapi.PTZTurnLeftLower,
	"turn_right_upper": univiewapi.PTZTurnRightUpper,
	"turn_right_lower": univiewapi.PTZTurnRightLower,
	"zoom_in":          univiewapi.PTZZoomIn,
	"zoom_out":         univiewapi.PTZZoomOut,
	"focus_near":       univiewapi.PTZFocusNear,
	"focus_far":        univiewapi.PTZFocusFar,
	"iris_increase":    univiewapi.PTZIrisIncrease,
	"iris_decrease":    univiewapi.PTZIrisDecrease,
	"wiper_on":         univiewapi.PTZWiperOn,
	"wiper_off":        univiewapi.PTZWiperOff,
	"light_on":         univiewapi.PTZLightOn,
	"light_off":        univiewapi.PTZLightOff,
	"heater_on":        univiewapi.PTZHeaterOn,
	"heater_off":       univiewapi.PTZHeaterOff,
	"ir_on":            univiewapi.PTZIROn,
	"ir_off":           univiewapi.PTZIROff,
	"stop":             univiewapi.PTZStop,
}

func (h *handler) ptzCtrl(msg websocket.Message) ([]byte, error) {
	args, err := json.Marshal(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), nil
	}

	req := &ptzCtrlMsg{}
	if err := json.Unmarshal(args, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	cam, err := h.device.GetCamera(req.CameraID)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	if cam.GetBrand() == utils.Uniview {
		aiCam, _ := cam.(*uniview.BaseUniviewCamera)

		channel := aiCam.GetChannel()
		nc, err := h.device.GetNVRManager().GetNVRClientBySN(aiCam.GetNvrSN())
		if err != nil {
			return msg.ReplyMessage(err).Marshal(), err
		}
		cmd, ok := ptzCmdMap[req.Command]
		if !ok {
			cmd = univiewapi.PTZStop
		}
		err = nc.PTZCtrl(channel, cmd, req.HSpeed, req.VSpeed)
		return msg.ReplyMessage(err).Marshal(), err
	}
	return nil, err
}

func (h *handler) getPTZPresets(msg websocket.Message) ([]byte, error) {
	args, err := json.Marshal(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), nil
	}
	req := &getPtzPresetsReq{}
	if err := json.Unmarshal(args, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	cam, err := h.device.GetCamera(req.CameraID)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	if cam.GetBrand() == utils.Uniview {
		aiCam, _ := cam.(*uniview.BaseUniviewCamera)

		channel := aiCam.GetChannel()
		nc, err := h.device.GetNVRManager().GetNVRClientBySN(aiCam.GetNvrSN())
		if err != nil {
			return msg.ReplyMessage(err).Marshal(), err
		}

		presets, err := nc.GetPTZPresets(channel)
		if err != nil {
			return msg.ReplyMessage(err).Marshal(), err
		}
		ret := getPtzPresetsRet{Num: presets.Nums}
		for _, v := range presets.PresetInfos {
			ret.Presets = append(ret.Presets, ptzPreset{ID: v.ID, Name: v.Name})
		}
		return msg.ReplyMessage(ret).Marshal(), nil
	}
	return msg.ReplyMessage(nil).Marshal(), err
}

func (h *handler) setPTZPreset(msg websocket.Message) ([]byte, error) {
	args, err := json.Marshal(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), nil
	}
	req := &setPtzPresetReq{}
	if err := json.Unmarshal(args, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	cam, err := h.device.GetCamera(req.CameraID)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	if cam.GetBrand() == utils.Uniview {
		aiCam, _ := cam.(*uniview.BaseUniviewCamera)
		channel := aiCam.GetChannel()

		nc, err := h.device.GetNVRManager().GetNVRClientBySN(aiCam.GetNvrSN())
		if err != nil {
			return msg.ReplyMessage(err).Marshal(), err
		}
		preset := univiewapi.PresetInfo{
			ID:   req.Preset.ID,
			Name: req.Preset.Name,
		}
		err = nc.PutPTZPreset(channel, uint32(req.Preset.ID), &preset)
		if err != nil {
			return msg.ReplyMessage(err).Marshal(), err
		}
	}
	return msg.ReplyMessage(nil).Marshal(), err
}

func (h *handler) goToPTZPreset(msg websocket.Message) ([]byte, error) {
	args, err := json.Marshal(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), nil
	}
	req := &goToPtzPresetReq{}
	if err := json.Unmarshal(args, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	cam, err := h.device.GetCamera(req.CameraID)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	if cam.GetBrand() == utils.Uniview {
		aiCam, _ := cam.(*uniview.BaseUniviewCamera)
		channel := aiCam.GetChannel()

		nc, err := h.device.GetNVRManager().GetNVRClientBySN(aiCam.GetNvrSN())
		if err != nil {
			return msg.ReplyMessage(err).Marshal(), err
		}
		err = nc.PutPTZPresetGoTo(channel, req.PresetID)
		if err != nil {
			return msg.ReplyMessage(err).Marshal(), err
		}
	}
	return msg.ReplyMessage(nil).Marshal(), err
}
