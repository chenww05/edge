package box

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/icholy/digest"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog"

	univiewapi "github.com/example/goshawk/uniview"
	"github.com/example/onvif"
	"github.com/example/turing-common/aes"
	"github.com/example/turing-common/log"
	"github.com/example/turing-common/metrics"
	"github.com/example/turing-common/model"
	"github.com/example/turing-common/websocket"

	"github.com/example/minibox/camera/base"
	"github.com/example/minibox/camera/thermal_1"
	"github.com/example/minibox/camera/uniview"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/discover"
	"github.com/example/minibox/discover/arp"
	"github.com/example/minibox/pem"
	"github.com/example/minibox/scheduler"
	"github.com/example/minibox/stream"
	"github.com/example/minibox/utils"
)

const (
	GetCameraTimeZone             = "box.camera.get_camera_timezone"
	SetCameraTimezone             = "box.camera.set_camera_timezone"
	SetCameraBackground           = "box.camera.set_camera_background"
	SetBatchCameraBackground      = "box.camera.set_batch_camera_background"
	CheckCameraBackground         = "box.camera.check_camera_background_hash"
	GetUploadConfig               = "box.get_upload_config"
	SetUploadConfig               = "box.set_upload_config"
	SetTimeZone                   = "box.set_timezone"
	SdpTransport                  = "box.camera.sdp_transport"
	StartStream                   = "box.camera.start_stream"
	StartWebStream                = "box.camera.start_web_stream"
	RecordVideo                   = "box.camera.record_video"
	Search                        = "box.search"
	ValidateCamera                = "box.validate_camera"
	ValidateDvr                   = "box.validate_dvr"
	DebugEcho                     = "box.debug_echo"
	ConfigChanged                 = "config.changed"
	InitBox                       = "box.init"
	UpdateCamera                  = "box.camera.update"
	PTZCtrl                       = "box.camera.ptz_ctrl"
	GetPTZPresets                 = "nest.box.camera.get_ptz_presets"
	SetPTZPreset                  = "nest.box.camera.set_ptz_preset"
	GoToPTZPreset                 = "nest.box.camera.go_to_ptz_preset"
	GetRecordsLegacy              = "box.camera.get_records"
	GetRecords                    = "nest.box.camera.get_records"
	GetRecordsDaily               = "nest.box.camera.get_records_daily"
	StreamAction                  = "box.camera.stream_action"
	ArchiveSetting                = "box.camera.archive_setting"
	StreamSettings                = "box.camera.stream_settings"
	GetNvrSDInfos                 = "box.get_nvr_sd_infos"
	SetImageEnhance               = "box.camera.set_image_enhance"
	GetImageEnhance               = "box.camera.get_image_enhance"
	SetNvrUserInfo                = "nest.box.set_nvr_user_info"
	LockNvrClient                 = "nest.box.nvr.lock"
	ActivateNvr                   = "box.activate_nvr"
	GetActivateStatus             = "box.get_nvr_activate_status"
	SetNvrDeviceName              = "nest.box.nvr.set_name"
	UpdateCameraSettings          = "nest.box.camera.update_settings"
	UpdateCameraSettingsOld       = "box.update_camera_settings"
	StartBackwardAudio            = "box.camera.start_backward_audio"
	StopBackwardAudio             = "box.camera.stop_backward_audio"
	HeartbeatBackwardAudio        = "box.camera.heartbeat_backward_audio"
	UpdateBoxSetting              = "box.update_box_setting"
	IotDiscover                   = "nest.box.discover_devices"
	StartMultiStream              = "nest.box.camera.start_multi_streams"
	StartMultiCastStream          = "nest.box.camera.start_multi_cast_streams"
	IotPlayAudioClip              = "nest.box.iot.play_audio_clip"
	IotStopAudioClip              = "nest.box.iot.stop_audio_clip"
	IotValidateSpeaker            = "nest.box.iot.validate_speaker"
	NestPtzCtrl                   = "nest.box.camera.ptz_ctrl"
	NestStreamAction              = "nest.box.camera.stream.action"
	NestStartBackwardAudio        = "nest.box.camera.backward_audio.start"
	NestStopBackwardAudio         = "nest.box.camera.backward_audio.stop"
	NestHeartbeatAudio            = "nest.box.camera.backward_audio.heartbeat"
	NestGeneralStartBackwardAudio = "nest.box.general.backward_audio.start"
	NestGeneralStopBackwardAudio  = "nest.box.general.backward_audio.stop"
	NestGeneralHeartbeatAudio     = "nest.box.general.backward_audio.heartbeat"
)

const (
	defaultDurationForChangePassword = 10
	defaultPlayPath                  = "rtc/v1/play"
	defaultDataPath                  = "rtc/v1/data"
	sdpH265KeyWord                   = "webrtc-datachannel"
	axisPlayClipUrl                  = "http://%s/axis-cgi/playclip.cgi?clip=%d&volume=%d"
	axisStopClipUrl                  = "http://%s/axis-cgi/stopclip.cgi"
	axisListClipUrl                  = "http://%s/axis-cgi/param.cgi?action=list&group=MediaClip"
)

const (
	ChannelStatusConnecting          = 0
	ChannelStatusOnline              = 1
	ChannelStatusIncorrectLoginInfo  = 2
	ChannelStatusNetworkDisconnected = 3
)

const (
	NvrActivated          string = "Activated"
	NvrInactivated        string = "Inactivated"
	UnknownActivateStatus string = "Unknown"
)

const (
	AudioEncodeTypeG711U string = "mulaw"
	AudioEncodeTypeG711A string = "alaw"
)

var availableDirs = [...]string{"/home/res/image", "/home/FaceUI/icons"}

var ErrDirInvalid = errors.New("invalid file upload path")
var ErrArgsWrongType = errors.New("args wrong field type")
var ErrArgsValidation = errors.New("args validation error")
var ErrIncompatibleCamera = errors.New("wrong camera type")
var ErrInvalid = errors.New("invalid")
var ErrInvalidAICamera = errors.New("it's a valid AI camera")
var ErrBackwardAudioOngoing = errors.New("Someone is talking down to the camera right now，please try again later")
var ErrNotBackwardAudioCamera = errors.New("Backward audio doesn't been supported int this camera")
var ErrBackwardAudioNotStarted = errors.New("Backward audio has not been started")
var ErrBackwardAudioHasStopped = errors.New("Backward audio has been stopped")
var ErrHttpAuthFailed = errors.New("The username or password is incorrect")

// websocket handler
type handler struct {
	log               zerolog.Logger
	registeredActions map[string]func(websocket.Message) ([]byte, error)
	device            Box
	searcher          *discover.SearcherProcessor
}

func NewHandler(device Box) websocket.Handler {
	h := &handler{log: log.Logger("Handle"), device: device, searcher: device.GetSearcher()}
	h.registerActions()
	return h
}

func (h *handler) Handle(payload []byte) ([]byte, error) {
	msg, err := websocket.ToMessage(payload)
	if err != nil {
		h.log.Error().Msgf("ToMessage err: %s, content: %s", err, string(payload))
		return msg.ReplyMessage(err).Marshal(), err
	}

	actionFunc, ok := h.registeredActions[msg.GetAction()]
	if ok {
		data, err := actionFunc(msg)
		go metrics.WSCounterCollect(msg.GetAction(), err)
		return data, err
	} else {
		return h.unknownAction(msg)
	}
}

func (h *handler) registerActions() {
	actions := map[string]func(websocket.Message) ([]byte, error){
		GetCameraTimeZone:        h.getCameraTimeZone,
		SetCameraTimezone:        h.setCameraTimeZone,
		SetCameraBackground:      h.setCameraBackground,
		SetBatchCameraBackground: h.setBatchCameraBackground,
		CheckCameraBackground:    h.checkCameraBackground,
		GetUploadConfig:          h.getUploadConfig,
		SetUploadConfig:          h.setUploadConfig,
		SetTimeZone:              h.setBoxTimeZone,
		SdpTransport:             h.sdpTransport,
		StartStream:              h.startStream,
		StartWebStream:           h.startStream,
		StartMultiStream:         h.startMultiStream,
		StartMultiCastStream:     h.startMultiStream,
		RecordVideo:              h.recordVideo,
		Search:                   h.search,
		ValidateDvr:              h.validateDvr,
		ValidateCamera:           h.validateCamera,
		DebugEcho:                h.debugEcho,
		ConfigChanged:            h.configChanged,
		InitBox:                  h.initBox,
		UpdateCamera:             h.updateCameraFromCloud,
		GetRecordsLegacy:         h.getRecords,
		GetRecords:               h.getRecords,
		GetRecordsDaily:          h.getRecordsDaily,
		PTZCtrl:                  h.ptzCtrl,
		GetPTZPresets:            h.getPTZPresets,
		SetPTZPreset:             h.setPTZPreset,
		GoToPTZPreset:            h.goToPTZPreset,
		StreamAction:             h.streamAction,
		ArchiveSetting:           h.handleArchiveSettings,
		StreamSettings:           h.streamSettings,
		GetNvrSDInfos:            h.getNvrSDInfos,
		SetImageEnhance:          h.handleSetImageEnhance,
		GetImageEnhance:          h.handleGetImageEnhance,
		SetNvrUserInfo:           h.setNvrUserInfo,
		LockNvrClient:            h.lockNvrClient,
		ActivateNvr:              h.activateNvr,
		GetActivateStatus:        h.getActivateStatus,
		SetNvrDeviceName:         h.setNvrName,
		UpdateCameraSettings:     h.refreshCameraSettings,
		UpdateCameraSettingsOld:  h.refreshCameraSettings,
		StartBackwardAudio:       h.startBackwardAudio,
		StopBackwardAudio:        h.stopBackwardAudio,
		HeartbeatBackwardAudio:   h.heartBackwardAudio,
		UpdateBoxSetting:         h.handleBoxSetting,

		// For ws msg from nest
		IotDiscover:                   h.iotDiscover,
		IotPlayAudioClip:              h.iotPlayAudioClip,
		IotStopAudioClip:              h.iotStopAudioClip,
		IotValidateSpeaker:            h.iotValidateSpeaker,
		NestStreamAction:              h.streamAction,
		NestPtzCtrl:                   h.ptzCtrl,
		NestStartBackwardAudio:        h.startBackwardAudio,
		NestHeartbeatAudio:            h.heartBackwardAudio,
		NestStopBackwardAudio:         h.stopBackwardAudio,
		NestGeneralStartBackwardAudio: h.startGeneralBackwardAudio,
		NestGeneralStopBackwardAudio:  h.stopGeneralBackwardAudio,
		NestGeneralHeartbeatAudio:     h.heartbeatGeneralBackwardAudio,
	}
	h.registeredActions = actions
}

func (h *handler) unknownAction(msg websocket.Message) ([]byte, error) {
	err := fmt.Errorf("do not know how to handle msg of action %v: %s", msg.GetAction(), string(msg.Marshal()))
	return nil, err
}

func (h *handler) configChanged(msg websocket.Message) ([]byte, error) {
	h.log.Info().Msg("configChanged called")
	err := h.device.UpdateCameras()
	if err != nil {
		err = fmt.Errorf("init cameras error %v, %s", err, string(msg.Marshal()))
	}
	return nil, err
}

func (h *handler) initBox(msg websocket.Message) ([]byte, error) {
	// err := h.device.Init()
	// if err != nil {
	//	err = fmt.Errorf("box init error %v, %s", err, string(msg.Marshal()))
	// }
	// return nil, err
	return nil, nil
}

func (h *handler) debugEcho(msg websocket.Message) ([]byte, error) {
	return msg.ReplyMessage(msg.GetArgs()).Marshal(), nil
}

func (h *handler) validateCamera(msg websocket.Message) ([]byte, error) {
	args, err := json.Marshal(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), nil
	}
	req := &validateCamReq{}
	if err := json.Unmarshal(args, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	req.Username, req.Password, req.Host = utils.ParserUri(req.Uri)
	if req.Host == "" {
		return msg.ReplyMessage(ErrInvalid).Marshal(), nil
	}

	dev := h.searcher.GetDeviceByHost(req.Host)
	brand := utils.GetCameraBrand(dev.Info.Manufacturer)
	_, port := utils.ParseXAddr(dev.Params.Xaddr)
	nc := univiewapi.NewNvrClient(req.Host, req.Username, req.Password, port,
		univiewapi.WithHttpClientWrap(metrics.HttpClientCounterCollect),
		univiewapi.WithLogger(log.Logger("handler")))

	snapshot, err := nc.GetChannelStreamSnapshot(1, 1)
	if err != nil {
		h.log.Err(err).Msg("get snapshot error")
		return msg.ReplyMessage(ErrInvalid).Marshal(), nil
	}
	if len(snapshot) == 0 {
		h.log.Warn().Msg("get snapshot nil")
		return msg.ReplyMessage(ErrInvalid).Marshal(), nil
	}

	s3File := h.uploadSnapshotToS3(snapshot)
	if s3File == nil {
		h.log.Warn().Str("act", ValidateCamera).Msgf("update s3 result nil")
		return msg.ReplyMessage(ErrDirInvalid).Marshal(), nil
	}
	rtspPort := utils.DefaultRtspPort
	portInfo, err := nc.GetNetworkPort()
	if err != nil {
		h.log.Error().Str("act", ValidateCamera).Msgf("get network port failed")
	} else {
		rtspPort = portInfo.RTSPPort
	}
	sdUri, uri, hdUri := utils.GetStreamUri(brand, req.Host, rtspPort, 1)
	data := validateCamRet{
		MacAddress: dev.Info.MACAddress,
		SDUri:      sdUri,
		Uri:        uri,
		HDUri:      hdUri,
		Snapshot:   *s3File,
	}

	buff, _ := json.Marshal(data)
	h.log.Info().RawJSON("data", buff).Str("act", ValidateCamera).Msg("reply")
	return msg.ReplyMessage(data).Marshal(), nil
}

func (h *handler) validateDvr(msg websocket.Message) ([]byte, error) {
	args := msg.GetArgs()
	argsData, _ := json.Marshal(args["data"])
	req := &validateDvrReq{}
	if err := json.Unmarshal(argsData, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	dev := h.searcher.GetDeviceByHost(req.Host)
	brand := utils.GetCameraBrand(dev.Info.Manufacturer)
	_, port := utils.ParseXAddr(dev.Params.Xaddr)
	cameras := make([]validateDvrCameraRet, 0)
	wg := sync.WaitGroup{}
	nc := univiewapi.NewNvrClient(req.Host, req.Username, req.Password, port,
		univiewapi.WithHttpClientWrap(metrics.HttpClientCounterCollect),
		univiewapi.WithLogger(log.Logger("handler")),
	)
	deviceInfos, err := nc.GetDeviceInfos()
	if err != nil {
		h.log.Err(err).Msg("get device error")
		return msg.ReplyMessage(ErrInvalid).Marshal(), err
	}

	channelDetailInfos, err := nc.GetChannelDetailInfos()
	if err != nil {
		h.log.Err(err).Msg("get channel detail error")
		return msg.ReplyMessage(ErrInvalid).Marshal(), err
	}

	if (channelDetailInfos.Nums <= 0 || len(channelDetailInfos.DetailInfos) == 0) && (deviceInfos.Nums <= 0 || len(deviceInfos.DeviceInfos) == 0) {
		h.log.Warn().Msgf("no devices")
		data := validateDvrRet{
			Cameras: cameras,
		}

		buff, _ := json.Marshal(data)
		h.log.Info().RawJSON("data", buff).Str("act", ValidateDvr).Msg("reply")
		return msg.ReplyMessage(data).Marshal(), nil
	}

	h.device.UpdateNVR(dev.Info.SerialNumber, req.Username, req.Password, brand, req.Host, port)

	channelToInfo := make(map[uint32]univiewapi.DetailInfo)

	for _, detailInfo := range channelDetailInfos.DetailInfos {
		channelToInfo[detailInfo.ID] = detailInfo
	}

	allDeviceInfos := make(map[uint32]univiewapi.DeviceInfo)
	for _, deviceInfo := range deviceInfos.DeviceInfos {
		allDeviceInfos[deviceInfo.ID] = deviceInfo
	}
	rtspPort := utils.DefaultRtspPort
	portInfo, err := nc.GetNetworkPort()
	if err != nil {
		h.log.Error().Str("act", ValidateDvr).Msgf("get network port failed")
	} else {
		rtspPort = portInfo.RTSPPort
	}

	snapshotMap := make(map[uint32][]byte)

	if utils.IsUniviewXVR(dev.Info) {
		snapshotMap = h.getXVRSnapshots(channelToInfo, req.Host, rtspPort, req.Username, req.Password)
	}

	// XVR device infos are empty
	for _, deviceInfo := range allDeviceInfos {
		id := deviceInfo.ID
		// only try to get snapshot if camera online
		detailInfo, ok := channelToInfo[id]
		if !ok {
			continue
		}
		if detailInfo.OffReason != ChannelStatusOnline {
			continue
		}
		snapshot, err := nc.GetChannelStreamSnapshot(id, 1)
		if err != nil {
			h.log.Err(err).Str("act", ValidateDvr).Msg("get snapshot error")
			continue
		}
		if len(snapshot) > 0 {
			snapshotMap[id] = snapshot
		}
	}

	mu := sync.Mutex{}
	for _, detailInfo := range channelToInfo {

		channelStatus := detailInfo.OffReason

		// compatible with previous APP
		if channelStatus != ChannelStatusOnline && !req.WithChannelStatus {
			continue
		}

		// check XVR analog camera plugin status
		if utils.IsUniviewXVR(dev.Info) && detailInfo.Status != ChannelStatusOnline {
			continue
		}

		channelName := detailInfo.Name
		channelId := detailInfo.ID
		manufacturer := detailInfo.Manufacturer
		deviceModel := detailInfo.DeviceModel
		serialNumber := ""

		if utils.IsUniviewXVR(dev.Info) {
			manufacturer = utils.TuringXVR
			deviceModel = utils.XVRCamera
			if brand == "" {
				brand = utils.Uniview
			}
			// XVR serial + channelId
			serialNumber = fmt.Sprintf(utils.OEMSnFormat, dev.Info.SerialNumber, channelId)
		} else {
			if deviceInfo, ok := allDeviceInfos[channelId]; ok {
				// NVR
				if detailInfo.RemoteIndex > 1 {
					serialNumber = fmt.Sprintf(utils.OEMSnFormat, deviceInfo.SerialNumber, detailInfo.RemoteIndex)
				} else if utils.IsReolinkCamera(utils.GetReolinkCameraModel(deviceModel)) ||
					utils.IsDWCamera(utils.GetDWCameraModel(deviceModel)) || utils.IsAvyconCamera(utils.GetAvyconCameraModel(deviceModel)) {
					serialNumber = detailInfo.AddressInfo.MAC
				} else {
					serialNumber = deviceInfo.SerialNumber
				}
				if len(deviceInfo.DeviceModel) > 0 {
					deviceModel = deviceInfo.DeviceModel
				}
			}
		}
		snapshot := make([]byte, 0)
		if snapshotData, ok := snapshotMap[channelId]; ok {
			snapshot = snapshotData
		}

		sdUri, uri, hdUri := utils.GetStreamUri(brand, req.Host, rtspPort, channelId)

		wg.Add(1)
		go func() {
			defer wg.Done()

			// fill the validation return data
			camRet := validateDvrCameraRet{
				ChannelName:   channelName,
				ChannelId:     channelId,
				ChannelStatus: uint32(channelStatus),
				MacAddress:    dev.Info.MACAddress,
				NvrSN:         dev.Info.SerialNumber,
				SDUri:         sdUri,
				Uri:           uri,
				HDUri:         hdUri,
				SerialNo:      serialNumber,
				Brand:         string(brand),
				Manufacturer:  manufacturer,
				Model:         deviceModel,
			}

			// upload snapshot to S3
			if len(snapshot) > 0 {
				if s3File := h.uploadSnapshotToS3(snapshot); s3File != nil {
					camRet.Snapshot = s3File
				}
			}
			mu.Lock()
			cameras = append(cameras, camRet)
			mu.Unlock()
		}()
	}

	wg.Wait()

	data := validateDvrRet{
		Cameras: cameras,
	}
	buff, _ := json.Marshal(data)
	h.log.Info().RawJSON("data", buff).Str("act", ValidateDvr).Msg("reply")
	return msg.ReplyMessage(data).Marshal(), nil
}

func (h *handler) getXVRSnapshots(channels map[uint32]univiewapi.DetailInfo, host string, port uint16, username string, password string) map[uint32][]byte {
	// check XVR analog camera plugin status
	snapshotMap := make(map[uint32][]byte)
	for _, channelInfo := range channels {
		if channelInfo.Status != ChannelStatusOnline {
			continue
		}
		filePath := filepath.Join(h.device.GetConfig().GetDataStoreDir(), fmt.Sprintf("cam_snap_%s.jpg", time.Now().Format(time.RFC3339)))
		uri, err := utils.GetLiveUrl(fmt.Sprintf(utils.UniviewUriPattern, host, port, channelInfo.ID, 1), username, password)
		if err != nil {
			h.log.Err(err).Msg("Get liveUrl error: %s")
		}
		filePath, err = utils.GetLiveViewSnapshotFile(uri, filePath)
		if err != nil {
			h.log.Error().Msgf("Get live view snapshot error: %s", err)
			continue
		}
		snapshot, err := h.readXVRSnapshot(filePath)
		if err != nil {
			h.log.Error().Msgf("Upload snapshot path error: %s", err)
			continue
		}

		if len(snapshot) > 0 {
			snapshotMap[channelInfo.ID] = snapshot
		}
	}
	return snapshotMap
}

func (h *handler) readXVRSnapshot(filename string) ([]byte, error) {

	defer func() {
		if err := os.Remove(filename); err != nil {
			h.log.Error().Msgf("Failed to delete temp snapshot image file %v", err)
		}
	}()
	snapshot, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (h handler) uploadSnapshotToS3(snapshot []byte) *utils.S3File {
	file, err := ioutil.TempFile(h.device.GetConfig().GetDataStoreDir(), fmt.Sprintf("%s*.jpg", time.Now().Format(time.RFC3339)))
	if err != nil {
		h.log.Err(err).Msg("temp file error")
		return nil
	}
	defer file.Close()

	_, err = file.Write(snapshot)
	if err != nil {
		h.log.Err(err).Msg("file write error")
		return nil
	}
	filename := file.Name()

	h.log.Info().Str("filename", filename).Msg("saved temp snapshot file")
	defer func() {
		if err := os.Remove(filename); err != nil {
			h.log.Err(err).Str("filename", filename).Msg("unable to delete snapshot file")
		}
	}()
	// CameraId use 0 which must be a fake camera id, in order to get token by box
	s3File, err := h.device.UploadS3ByTokenName(0, filename, 0, 0, "jpg", TokenNameCameraSnap) // TODO 0,0 is bad
	if err != nil {
		h.log.Err(err).Str("filename", filename).Msg("UploadS3ByTokenName error")
		return nil
	}
	return s3File
}

func (h handler) search(msg websocket.Message) ([]byte, error) {
	deviceList := h.searcher.StartSearch()
	searchDevices := make([]device, 0)
	for _, v := range deviceList {
		if v.Info.Manufacturer == "" {
			continue
		}
		manufacturer := utils.GetCameraBrand(v.Info.Manufacturer)
		if !utils.IsInCameraBrandList(manufacturer) {
			h.log.Debug().Msgf("The dvr manufacturer brand(%s) not in we list %v", manufacturer, utils.BrandList)
			continue
		}
		ip, port := utils.ParseXAddr(v.Params.Xaddr)
		nvrClient := univiewapi.NewNoAuthNvrClient(ip, port,
			univiewapi.WithHttpClientWrap(metrics.HttpClientCounterCollect),
			univiewapi.WithLogger(h.log))
		status, _ := nvrClient.SecurityActivateStatus()
		activateStatus := UnknownActivateStatus
		if status != nil {
			activateStatus = status.Status
		}

		// No password when search nvr, so give the default rtsp port 554
		_, uri, _ := utils.GetStreamUri(manufacturer, ip, utils.DefaultRtspPort, 1)
		dev := device{
			Host:            ip,
			MacAddress:      v.Info.MACAddress,
			Manufacturer:    string(manufacturer),
			Model:           v.Info.Model,
			FirmwareVersion: v.Info.FirmwareVersion,
			SerialNo:        v.Info.SerialNumber,
			HardwareId:      v.Info.HardwareId,
			StreamUrl:       uri,
			ActivateStatus:  activateStatus,
		}

		if utils.IsUniviewXVR(v.Info) {
			dev.ActivateStatus = NvrActivated
		}

		searchDevices = append(searchDevices, dev)
	}

	data := searchDevice{
		Devices: searchDevices,
	}
	buff, _ := json.Marshal(data)
	h.log.Info().RawJSON("data", buff).Msg("reply")
	return msg.ReplyMessage(data).Marshal(), nil
}

type RecordVideoArg struct {
	TaskId      string `json:"task_id"`
	CameraID    int    `json:"camera_id"`
	Resolution  string `json:"resolution,omitempty"`
	StartedAt   int64  `json:"started_at"`
	EndedAt     int64  `json:"ended_at"`
	EnableAudio bool   `json:"enable_audio"`
}

func (h *handler) recordVideo(msg websocket.Message) ([]byte, error) {
	args := msg.GetArgs()
	baseCam, err := h.getCameraFromArgs(args)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	_, ok := baseCam.(base.AICamera)
	if !ok {
		h.log.Error().Err(ErrIncompatibleCamera).Msg("record failed")
		return msg.ReplyMessage(ErrIncompatibleCamera).Marshal(), ErrIncompatibleCamera
	}

	var req RecordVideoArg
	argsData, _ := json.Marshal(args)
	if err := json.Unmarshal(argsData, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	if req.StartedAt >= req.EndedAt {
		err = errors.New("The ended_at must not be less than started_at. ")
		return msg.ReplyMessage(err).Marshal(), err
	}
	if req.EndedAt-req.StartedAt > MaxRecordVideoDuration {
		err = fmt.Errorf("'The specified duration exceeds the maximum %d seconds. ", MaxRecordVideoDuration)
		return msg.ReplyMessage(err).Marshal(), err
	}
	p := GetRecordVideoProcess(h.device)
	if err := p.Start(req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	h.log.Info().RawJSON("data", argsData).Msg("reply")
	return msg.ReplyMessage(args).Marshal(), nil
}

func (h handler) sdpTransport(msg websocket.Message) ([]byte, error) {
	var sdp sdpTransportReq
	if err := json.Unmarshal(msg.Marshal(), &sdp); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	mValidator := validator.New()
	if err := mValidator.Struct(sdp); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	srsPort := h.device.GetConfig().GetStreamConfig().SrsApiPort

	srsIp, err := h.device.GetSrsIp()
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	sdpRemote, err := h.device.GetSdpRemote(srsIp, srsPort, sdp.Arg.SdpLocal, sdp.Arg.CameraId, sdp.Arg.StreamId)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	return msg.ReplyMessage(sdpResponse{CameraId: sdp.Arg.CameraId,
		StreamId: sdp.Arg.StreamId, SdpRemote: sdpRemote}).Marshal(), nil
}

func (h handler) startStream(msg websocket.Message) ([]byte, error) {
	var srp streamRequestParam
	if err := json.Unmarshal(msg.Marshal(), &srp); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	mValidator := validator.New()
	if err := mValidator.Struct(srp); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	cam, err := h.getCameraFromArgs(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	var baseUri, inputUri, outputUri, streamType, streamId string
	baseUri = srp.Arg.Token.BaseUri
	streamId = srp.Arg.StreamParam.StreamId
	outputUri = h.getStreamOutputUri(srp.Arg.StreamParam.StreamType, srp.Arg.Token)
	streamType = h.getStreamType(srp.Arg.StreamParam.StreamType)
	resolution := srp.Arg.StreamParam.Resolution
	// Fisheye camera may have main stream only in modes like fisheye ...
	if cam.GetCacheStreamNumber() == 1 {
		resolution = string(utils.HD)
	}
	inputUri, err = h.getStreamInputUri(cam, resolution, srp.Arg.StreamParam.StartTime, srp.Arg.StreamParam.EndTime, srp.Arg.StreamAction)
	if err != nil {
		h.log.Error().Err(err).Msg("failed to create input stream uri.")
		return msg.ReplyMessage(err).Marshal(), err
	}

	// getStreamEncodeInfo must be before StartStream for that the Web/Mobile player's first frame can be I frame
	encodeInfo := h.getStreamEncodeInfo(cam, resolution)

	outputUri, err = h.getStreamManager().StartStream(streamId, streamType, inputUri, outputUri)
	if err != nil {
		if err == scheduler.CommandDroppedError {
			return msg.ReplyMessage(websocket.Err{
				Code:           -1,
				DevelopMessage: scheduler.ResourceLimit,
				Message:        err.Error(),
			}).Marshal(), err
		} else {
			return msg.ReplyMessage(err).Marshal(), err
		}
	}

	h.log.Info().Str("streamid", streamId).Msgf("pull stream from %s and send to %s", inputUri, outputUri)
	return msg.ReplyMessage(streamResponse{BaseUri: baseUri, StreamType: streamType,
		StreamId: streamId, OutputUri: outputUri, EncodeInfo: encodeInfo}).Marshal(), err
}

func (h handler) startMultiStream(msg websocket.Message) ([]byte, error) {
	var srp streamMultiRequestParam
	if err := json.Unmarshal(msg.Marshal(), &srp); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	mValidator := validator.New()
	if err := mValidator.Struct(srp); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	var param []*streamInfo
	var errMulti error
	for k, v := range srp.Arg.Param {
		singleStream := &streamInfo{}
		param = append(param, singleStream)

		singleStream.CameraId = v.CameraId
		if v.Err.Code < 0 {
			h.log.Error().Msgf("stream index %v is error, msg = %v", k, v.Err.Message)
			singleStream.Err.Code = v.Err.Code
			singleStream.Err.Message = v.Err.Message
			continue
		}
		cam, err := h.getCameraFromCameraId(v.CameraId)
		if err != nil {
			singleStream.Err.Code = -1
			singleStream.Err.Message = err.Error()
			continue
		}

		singleStream.BaseUri = v.Token.BaseUri
		singleStream.StreamId = v.StreamId
		resolution := v.Resolution
		// Fisheye camera may have main stream only in modes like fisheye ...
		if cam.GetCacheStreamNumber() == 1 {
			resolution = string(utils.HD)
		}
		singleStream.Resolution = resolution
		singleStream.StartTime = v.StartTime

		rtmpUrl := h.getStreamOutputUri(v.StreamType, v.Token)
		singleStream.StreamType = h.getStreamType(v.StreamType)
		inputUri, err := h.getStreamInputUri(cam, resolution, v.StartTime, v.EndTime, v.StreamAction)
		if err != nil {
			h.log.Error().Err(err).Msg("failed to create input stream uri.")
			if errMulti != nil {
				errMulti = scheduler.OpenMultiStreamsError
			}
			singleStream.Err.Code = -1
			singleStream.Err.Message = err.Error()
			continue
		}

		encodeInfo := h.getStreamEncodeInfo(cam, resolution)
		rtmpUrl, err = h.getStreamManager().StartStream(singleStream.StreamId, singleStream.StreamType, inputUri, rtmpUrl)
		if err != nil {
			singleStream.Err.Code = -1
			singleStream.Err.Message = err.Error()
			continue
		}

		webrtcUrl := strings.Replace(singleStream.BaseUri, "rtmp", "webrtc", 1)
		webrtcUrl = strings.Replace(webrtcUrl, ":1935", "", 1)
		webrtcUrl = fmt.Sprintf("%s/%d/%s", webrtcUrl, singleStream.CameraId, singleStream.StreamId)

		var rtmpUri string
		if strings.Contains(rtmpUrl, stream.RtmpPrefix) {
			if srsIp, err := h.device.GetSrsIp(); err == nil {
				rtmpUri = fmt.Sprintf("%s/%d/%s", singleStream.BaseUri, singleStream.CameraId, singleStream.StreamId)
				rtmpUri = strings.Replace(rtmpUri, stream.RtmpLocalhost, srsIp, 1)
			}
		}

		h.log.Info().Msgf("pull stream from %s and send to %s, webrtc url is %s, rtmp url is %s", inputUri, rtmpUrl, webrtcUrl, rtmpUri)
		singleStream.OutputUri = webrtcUrl
		singleStream.RtmpUri = rtmpUri
		singleStream.EncodeInfo = encodeInfo
	}

	return msg.ReplyMessage(multistreamResponse{Param: param}).Marshal(), errMulti
}

// streamAction operate the existing streaming to pause, resume, seek and close.
func (h handler) streamAction(msg websocket.Message) ([]byte, error) {
	var sar streamActionReq
	err := mapstructure.Decode(msg.GetArgs(), &sar)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	mValidator := validator.New()
	if err = mValidator.Struct(sar); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	var status string
	switch stream.ActionType(sar.Action) {
	case stream.Pause:
		err = h.getStreamManager().PauseStream(sar.StreamId)
		status = stream.Paused
	case stream.Play:
		fallthrough
	case stream.Resume:
		err = h.getStreamManager().ResumeStream(sar.StreamId)
		status = stream.Resumed
	case stream.Seek:
		err = h.getStreamManager().SeekStream(sar.StreamId, int64(sar.Param))
		status = stream.Sought
	case stream.Scale:
		err = h.getStreamManager().ScaleStream(sar.StreamId, sar.Param)
		status = stream.Scaled
	case stream.Speed:
		err = h.getStreamManager().SpeedStream(sar.StreamId, sar.Param)
		status = stream.Speeded
	case stream.Stop:
		err = h.getStreamManager().StopStream(sar.StreamId)
		status = stream.Closed
	case stream.ForceKeyFrame:
		cam, err := h.getCameraFromArgs(msg.GetArgs())
		if nil == err {
			aiCam := cam.(base.AICamera)
			unvCam := aiCam.(*uniview.BaseUniviewCamera)
			streamId := uint32(sar.Param)
			err = unvCam.RequestKeyFrame(unvCam.GetChannel(), streamId, 0)
			if nil == err {
				h.log.Info().Msgf("%s force key frame succeed", sar.StreamId)
				status = stream.Forced
			}
		}
	default:
		return msg.ReplyMessage(stream.UnsupportedAction).Marshal(), stream.UnsupportedAction
	}

	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	return msg.ReplyMessage(streamActionResp{
		StreamId:     sar.StreamId,
		StreamStatus: status,
	}).Marshal(), err
}

// handleArchiveSettings handle the cloud settings action.
func (h handler) handleArchiveSettings(msg websocket.Message) ([]byte, error) {
	args, err := json.Marshal(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), nil
	}
	var css *model.CloudStorageSetting
	if err := json.Unmarshal(args, &css); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	if css.Id < 1 || css.CameraId < 1 {
		err = fmt.Errorf("invalid cloud storage setting param %v", css)
		return msg.ReplyMessage(err).Marshal(), err
	}

	atr := GetArchiveTaskRunner()
	if atr == nil {
		err = errors.New("box is not initialed yet")
		return msg.ReplyMessage(err).Marshal(), err
	}
	err = atr.UpdateArchiveSettings(css)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	return msg.ReplyMessage(nil).Marshal(), err
}

func (h handler) streamSettings(msg websocket.Message) ([]byte, error) {
	args := msg.GetArgs()
	var ssr streamSettingsReq
	err := mapstructure.Decode(args, &ssr)

	mValidator := validator.New()
	if err = mValidator.Struct(ssr); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	baseCam, err := h.getCameraFromArgs(args)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	switch baseCam.GetBrand() {
	case utils.Sunell:
	case utils.Uniview:
		// process audio enable
		aiCam, ok := baseCam.(base.AICamera)
		if !ok {
			err := errors.New("camera not support")
			return msg.ReplyMessage(err).Marshal(), err
		}
		nc, err := h.device.GetNVRManager().GetNVRClientBySN(aiCam.GetNvrSN())
		if err != nil {
			return msg.ReplyMessage(err).Marshal(), err
		}
		err = nc.SetAudioStatuses(aiCam.GetChannel(), []univiewapi.AudioStatus{
			{0, ssr.Audio1Enable},
			{1, ssr.Audio2Enable},
			{2, ssr.Audio3Enable},
		})
		return msg.ReplyMessage(err).Marshal(), err
	}
	return msg.ReplyMessage(nil).Marshal(), err
}

func (h handler) handleSetImageEnhance(msg websocket.Message) ([]byte, error) {
	var ieq *imageEnhanceSetReq
	err := mapstructure.Decode(msg.GetArgs(), &ieq)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	cam, err := h.device.GetCamera(ieq.CameraId)
	if err != nil {
		h.log.Error().Msgf("get camera error: %s", err)
		return msg.ReplyMessage(err).Marshal(), err
	}
	if cam.GetBrand() != utils.Uniview {
		return msg.ReplyMessage(errors.New("unsupported box type")).Marshal(), err
	}
	aiCam, ok := cam.(base.AICamera)
	if !ok {
		return msg.ReplyMessage(errors.New("wrong camera type")).Marshal(), err
	}
	nc, err := h.device.GetNVRManager().GetNVRClientBySN(aiCam.GetNvrSN())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	channelId := aiCam.GetChannel()
	if err = nc.PutImageEnhance(channelId, ieq.ImageRotation); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	return msg.ReplyMessage(nil).Marshal(), err
}

func (h handler) handleGetImageEnhance(msg websocket.Message) ([]byte, error) {
	var ieq *imageEnhanceGetReq
	err := mapstructure.Decode(msg.GetArgs(), &ieq)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	cam, err := h.device.GetCamera(ieq.CameraId)
	if err != nil {
		h.log.Error().Msgf("get camera error: %s", err)
		return msg.ReplyMessage(err).Marshal(), err
	}
	if cam.GetBrand() != utils.Uniview {
		return msg.ReplyMessage(errors.New("unsupported box type")).Marshal(), err
	}
	aiCam, ok := cam.(base.AICamera)
	if !ok {
		return msg.ReplyMessage(errors.New("wrong camera type")).Marshal(), err
	}
	nc, err := h.device.GetNVRManager().GetNVRClientBySN(aiCam.GetNvrSN())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	channelId := aiCam.GetChannel()
	ie, err := nc.GetImageEnhance(channelId)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	// convert to UnderScoreCase naming
	ret := imageEnhanceGetResp{
		Brightness:    ie.Brightness,
		Contrast:      ie.Contrast,
		Saturation:    ie.Saturation,
		Sharpness:     ie.Sharpness,
		ImageRotation: ie.ImageRotation,
		DNoiseReduce:  ie.DNoiseReduce,
		ImageOffset:   ie.ImageOffset,
	}
	return msg.ReplyMessage(ret).Marshal(), err
}

func (h handler) updateCameraFromCloud(msg websocket.Message) ([]byte, error) {
	args := msg.GetArgs()
	cam, err := h.getCameraFromArgs(args)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	var req utils.UpdateCameraReq
	argsData, _ := json.Marshal(args)
	if err := json.Unmarshal(argsData, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	aiCam, ok := cam.(base.AICamera)
	if !ok {
		// Not vision camera just update
		cam.UpdateCameraFromCloud(&req)
		return msg.ReplyMessage(nil).Marshal(), nil
	}

	// Vision camera check plugged and online firstly
	online := aiCam.GetOnline()
	plugged := h.device.GetNVRManager().IsCameraPlugged(aiCam)
	if plugged && online {
		cam.UpdateCameraFromCloud(&req)
	} else if !plugged {
		errMsg := fmt.Sprintf("cam %d not plugged", aiCam.GetID())
		h.log.Error().Msg(errMsg)
		err = errors.New(errMsg)
	} else if !online {
		errMsg := fmt.Sprintf("cam %d offline", aiCam.GetID())
		h.log.Error().Msg(errMsg)
		err = errors.New(errMsg)
	}
	// Fetch Nvrs from cloud when cloud device changed
	if cloudNvrs, nvrErr := h.device.CloudClient().GetNvrs(); nvrErr != nil {
		h.log.Error().Err(nvrErr).Msgf("get nvrs from cloud failed")
	} else {
		h.log.Debug().Msgf("get nvrs from cloud success, nvrs: %+v", cloudNvrs)
		h.device.SetCloudNvrs(cloudNvrs)
	}

	h.device.GetNVRManager().UpdateNvr()
	nc, _ := h.device.GetNVRManager().GetNVRClientBySN(aiCam.GetNvrSN())
	_ = aiCam.SetNVRClient(nc)

	return msg.ReplyMessage(nil).Marshal(), err
}

type CameraReboot interface {
	Reboot() error
}

func (h *handler) getUploadConfig(msg websocket.Message) ([]byte, error) {
	cfg := h.device.GetConfig()

	return msg.ReplyMessage(cfg.GetUploadConfig()).Marshal(), nil
}

func (h *handler) setUploadConfig(msg websocket.Message) ([]byte, error) {
	args := msg.GetArgs()
	cfg := h.device.GetConfig()
	upload := cfg.GetUploadConfig()

	if val, ok := args["disable_upload_picture"]; ok {
		if uploadPic, ok := val.(bool); ok {
			upload.DisableUploadPic = uploadPic
		} else {
			h.log.Error().Interface("disable_upload_picture", val).Msg("Set upload config: expected bool")
			return msg.ReplyMessage(ErrArgsWrongType).Marshal(), ErrArgsWrongType
		}
	}

	if val, ok := args["disable_upload_temperature"]; ok {
		if uploadTemp, ok := val.(bool); ok {
			upload.DisableUploadTemperature = uploadTemp
		} else {
			h.log.Error().Interface("disable_upload_temperature", val).Msg("Set upload config: expected bool")
			return msg.ReplyMessage(ErrArgsWrongType).Marshal(), ErrArgsWrongType
		}
	}

	if val, ok := args["disable_cloud"]; ok {
		if cloud, ok := val.(bool); ok {
			upload.DisableCloud = cloud
		} else {
			h.log.Error().Interface("disable_cloud", val).Msg("Set upload config: expected bool")
			return msg.ReplyMessage(ErrArgsWrongType).Marshal(), ErrArgsWrongType
		}
	}

	if val, ok := args["enable_gateway"]; ok {
		if enableGW, ok := val.(bool); ok {
			upload.EnableGateway = enableGW
		} else {
			h.log.Error().Interface("enable_gateway", val).Msg("Set upload config: expected bool")
			return msg.ReplyMessage(ErrArgsWrongType).Marshal(), ErrArgsWrongType
		}
	}

	if val, ok := args["gateway_upload_url"]; ok {
		if gw_url, ok := val.(string); ok {
			if _, err := url.Parse(gw_url); err == nil {
				upload.GatewayUploadUrl = gw_url
			} else {
				h.log.Error().Str("gateway_upload_url", gw_url).Msg("Set upload config: gateway_upload_url should be valid")
				return msg.ReplyMessage(ErrArgsValidation).Marshal(), ErrArgsValidation
			}
		} else {
			h.log.Error().Interface("gateway_upload_url", val).Msg("Set upload config: expected string")
			return msg.ReplyMessage(ErrArgsWrongType).Marshal(), ErrArgsWrongType
		}
	}

	if val, ok := args["camera_scanperiod_secs"]; ok {
		if p, ok := val.(float64); ok {
			period := int(p)
			if period >= 0 {
				upload.CameraScanPeriod = period
			} else {
				h.log.Error().Int("camera_scanperiod_secs", period).Msg("Set upload config: expected scanperiod >= 0")
				return msg.ReplyMessage(ErrArgsValidation).Marshal(), ErrArgsValidation
			}
		} else {
			h.log.Error().Interface("camera_scanperiod_secs", val).Msg("Set upload config: expected int")
			return msg.ReplyMessage(ErrArgsWrongType).Marshal(), ErrArgsWrongType
		}
	}

	err := h.device.SetUploadConfig(*upload)
	if err != nil {
		h.log.Error().Err(err).Msg("Can't save upload config to minibox")
		return msg.ReplyMessage(err).Marshal(), err
	}

	return msg.ReplyMessage(nil).Marshal(), nil
}

func (h *handler) setBoxTimeZone(msg websocket.Message) ([]byte, error) {
	args := msg.GetArgs()
	cfg := h.device.GetConfig()
	timezone, _ := args["timezone"].(string)
	err := cfg.SetTimeZone(timezone)
	if err == nil {
		h.device.GetNVRManager().SetTimeZone(timezone)
	}
	return nil, err
}

type timeZoneInfo struct {
	TimeZone string `json:"timezone"`
}

type TimeZoneDevice interface {
	GetTimeZone() (string, error)
	SetTimeZone(string) error
}

func (h *handler) getCameraTimeZone(msg websocket.Message) ([]byte, error) {
	args := msg.GetArgs()
	baseCam, err := h.getCameraFromArgs(args)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	data := timeZoneInfo{}

	cam, ok := baseCam.(TimeZoneDevice)
	if !ok {
		h.log.Error().Err(ErrIncompatibleCamera).Msg("Get camera config failed")
		return msg.ReplyMessage(ErrIncompatibleCamera).Marshal(), ErrIncompatibleCamera
	}

	timezone, err := cam.GetTimeZone()
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	data.TimeZone = timezone

	return msg.ReplyMessage(data).Marshal(), nil
}

func (h *handler) setCameraTimeZone(msg websocket.Message) ([]byte, error) {
	args := msg.GetArgs()
	baseCam, err := h.getCameraFromArgs(args)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	cam, ok := baseCam.(TimeZoneDevice)
	if !ok {
		h.log.Error().Err(ErrIncompatibleCamera).Msg("Set camera config failed")
		return msg.ReplyMessage(ErrIncompatibleCamera).Marshal(), ErrIncompatibleCamera
	}

	var req timeZoneInfo
	validate := validator.New()

	argsData, _ := json.Marshal(args)
	if err := json.Unmarshal(argsData, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	} else if err := validate.Struct(req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	err = cam.SetTimeZone(req.TimeZone)
	if err != nil {
		h.log.Error().Msgf("set timezone error: %s", err)
		return msg.ReplyMessage(err).Marshal(), err
	}

	err = h.setBoxTimeZoneByCloud(req.TimeZone)
	if err != nil {
		h.log.Error().Msgf("set timezone error: %s", err)
		return msg.ReplyMessage(err).Marshal(), err
	}

	return msg.ReplyMessage(nil).Marshal(), nil
}

func (h *handler) setCameraBackground(msg websocket.Message) ([]byte, error) {
	args := msg.GetArgs()
	baseCam, err := h.getCameraFromArgs(args)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	cam, ok := baseCam.(thermal_1.Thermal1Camera)
	if !ok {
		h.log.Error().Err(ErrIncompatibleCamera).Msg("Set camera questionnaire failed")
		return msg.ReplyMessage(ErrIncompatibleCamera).Marshal(), ErrIncompatibleCamera
	}

	var req thermal_1.BackgroundSettings
	validate := validator.New()

	argsData, _ := json.Marshal(args)
	if err := json.Unmarshal(argsData, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	} else if err := validate.Struct(req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	h.processBackgroundSettings(&req)

	err = cam.SetBackground(&req)
	if err != nil {
		h.log.Error().Msgf("set background error: %s", err)
		return msg.ReplyMessage(err).Marshal(), err
	}

	if err := cam.Reboot(); err != nil {
		h.log.Error().Err(err).Msg("Can't reboot camera")
		return msg.ReplyMessage(err).Marshal(), err
	}

	cameraID, _ := args["camera_id"]
	cameraId, _ := cameraID.(float64)
	resp := thermal_1.SetBackgroundResp{
		CameraID:                    int(cameraId),
		QuestionnaireDetailImgEnMd5: req.QuestionnaireDetailImgEnMd5,
		QuestionnaireDetailImgSpMd5: req.QuestionnaireDetailImgSpMd5,
		QuestionnaireOKImgEnMd5:     req.QuestionnaireOKImgEnMd5,
		QuestionnaireOKImgSpMd5:     req.QuestionnaireOKImgSpMd5,
		SplashMd5:                   req.SplashMd5,
	}
	return msg.ReplyMessage(resp).Marshal(), nil
}

func (h *handler) setBatchCameraBackground(msg websocket.Message) ([]byte, error) {
	args := msg.GetArgs()

	// process settings
	var req thermal_1.BackgroundSettings
	validate := validator.New()

	argsData, _ := json.Marshal(args)
	if err := json.Unmarshal(argsData, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	} else if err := validate.Struct(req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	h.processBackgroundSettings(&req)

	// process cam
	cameraIDs, ok := args["camera_ids"]
	if !ok {
		err := fmt.Errorf("arg camera_ids is required")
		h.log.Error().Msgf("error: %s", err)
		return msg.ReplyMessage(err).Marshal(), err
	}
	camIDs, ok := cameraIDs.([]interface{})
	if !ok {
		err := fmt.Errorf("arg camera_ids error")
		h.log.Error().Msgf("error: %s", err)
		return msg.ReplyMessage(err).Marshal(), err
	}
	var errCamIDs []int
	var okCamIDs []int
	var okCams []thermal_1.Thermal1Camera
	ch := make(chan string)
	for i := 0; i < len(camIDs); i++ {
		cameraId, ok := camIDs[i].(float64)
		if !ok {
			h.log.Error().Msgf("camera_id should be int %s", camIDs[i])
			errCamIDs = append(errCamIDs, int(cameraId))
			continue
		}
		baseCam, err := h.device.GetCamera(int(cameraId))
		if err != nil {
			h.log.Error().Msgf("Get camera error: %s", err)
			errCamIDs = append(errCamIDs, int(cameraId))
			continue
		}
		cam, ok := baseCam.(thermal_1.Thermal1Camera)
		if !ok {
			h.log.Error().Err(ErrIncompatibleCamera).Msg("Set camera questionnaire failed")
			errCamIDs = append(errCamIDs, int(cameraId))
			continue
		}
		okCams = append(okCams, cam)
	}
	if len(okCams) > 0 {
		go func() {
			wg := sync.WaitGroup{}
			wg.Add(len(okCams))
			for i := 0; i < len(okCams); i++ {
				cam := okCams[i]
				go func(cam thermal_1.Thermal1Camera) {
					err := cam.SetBackground(&req)
					if err != nil {
						h.log.Error().Msgf("set background error: %s", err)
						ch <- fmt.Sprintf("%d#failed", cam.GetID())
					} else {
						if err := cam.Reboot(); err != nil {
							h.log.Error().Err(err).Msg("Can't reboot camera")
							ch <- fmt.Sprintf("%d#failed", cam.GetID())
						} else {
							ch <- fmt.Sprintf("%d#ok", cam.GetID())
						}
					}
					wg.Done()
				}(cam)
			}
			wg.Wait()
			close(ch)
		}()

		for camStatus := range ch {
			camStatusSlice := strings.Split(camStatus, "#")
			camId, _ := strconv.Atoi(camStatusSlice[0])
			if camStatusSlice[1] == "ok" {
				okCamIDs = append(okCamIDs, camId)
			} else {
				errCamIDs = append(errCamIDs, camId)
			}
		}
	}

	resp := thermal_1.BatchSetBackgroundResp{
		ErrCameraIDs:                errCamIDs,
		OkCameraIDs:                 okCamIDs,
		QuestionnaireDetailImgEnMd5: req.QuestionnaireDetailImgEnMd5,
		QuestionnaireDetailImgSpMd5: req.QuestionnaireDetailImgSpMd5,
		QuestionnaireOKImgEnMd5:     req.QuestionnaireOKImgEnMd5,
		QuestionnaireOKImgSpMd5:     req.QuestionnaireOKImgSpMd5,
		SplashMd5:                   req.SplashMd5,
	}
	return msg.ReplyMessage(resp).Marshal(), nil
}

func (h *handler) checkCameraBackground(msg websocket.Message) ([]byte, error) {
	args := msg.GetArgs()
	// process cam
	cameraIDs, ok := args["camera_ids"]
	if !ok {
		err := fmt.Errorf("arg camera_ids is required")
		h.log.Error().Msgf("error: %s", err)
		return msg.ReplyMessage(err).Marshal(), err
	}
	camIDs, ok := cameraIDs.([]interface{})
	if !ok {
		err := fmt.Errorf("arg camera_ids error")
		h.log.Error().Msgf("error: %s", err)
		return msg.ReplyMessage(err).Marshal(), err
	}
	var errCamIDs []int
	okCamsDict := make(map[int]interface{})
	var okCams []thermal_1.Thermal1Camera
	ch := make(chan string)
	for i := 0; i < len(camIDs); i++ {
		cameraId, ok := camIDs[i].(float64)
		if !ok {
			h.log.Error().Msgf("camera_id should be int %s", camIDs[i])
			errCamIDs = append(errCamIDs, int(cameraId))
			continue
		}
		baseCam, err := h.device.GetCamera(int(cameraId))
		if err != nil {
			h.log.Error().Msgf("Get camera error: %s", err)
			errCamIDs = append(errCamIDs, int(cameraId))
			continue
		}
		cam, ok := baseCam.(thermal_1.Thermal1Camera)
		if !ok {
			h.log.Error().Err(ErrIncompatibleCamera).Msg("Set camera questionnaire failed")
			errCamIDs = append(errCamIDs, int(cameraId))
			continue
		}
		okCams = append(okCams, cam)
	}
	if len(okCams) > 0 {
		go func() {
			wg := sync.WaitGroup{}
			wg.Add(len(okCams))
			for i := 0; i < len(okCams); i++ {
				cam := okCams[i]
				go func(cam thermal_1.Thermal1Camera) {
					resp, err := cam.GetBackground()
					respData, _ := json.Marshal(resp)
					if err != nil {
						h.log.Error().Msgf("set background error: %s", err)
						ch <- fmt.Sprintf("%d#failed", cam.GetID())
					} else {
						ch <- fmt.Sprintf("%d#%s", cam.GetID(), respData)
					}
					wg.Done()
				}(cam)
			}
			wg.Wait()
			close(ch)
		}()

		for camStatus := range ch {
			camStatusSlice := strings.Split(camStatus, "#")
			camId, _ := strconv.Atoi(camStatusSlice[0])
			if camStatusSlice[1] == "failed" {
				errCamIDs = append(errCamIDs, camId)
			} else {
				var resp thermal_1.BackGroupResponse
				if err := json.Unmarshal([]byte(camStatusSlice[1]), &resp); err != nil {
					errCamIDs = append(errCamIDs, camId)
				} else {
					okCamsDict[camId] = resp
				}
			}
		}
	}

	resp := thermal_1.CheckBackgroundResp{
		ErrCameraIDs: errCamIDs,
		OkCamsDict:   okCamsDict,
	}
	return msg.ReplyMessage(resp).Marshal(), nil
}

func (h *handler) setBoxTimeZoneByCloud(timezone string) error {
	config := h.device.GetConfig()
	return config.SetTimeZone(timezone)
}

func isSubPath(path string, paths []string) bool {
	for _, p := range paths {
		if strings.HasPrefix(path, p) {
			subPath := strings.TrimPrefix(path, p)
			if subPath != "" && subPath != "/" {
				return true
			}
		}
	}

	return false
}

func (h *handler) getCameraFromCameraId(cameraId int64) (base.Camera, error) {
	cam, err := h.device.GetCamera(int(cameraId))
	if err != nil {
		h.log.Error().Msgf("get camera error: %s", err)
		return nil, err
	}
	return cam, nil
}

func (h *handler) getCameraFromArgs(args map[string]interface{}) (base.Camera, error) {
	cameraID, ok := args["camera_id"]
	if !ok {
		err := fmt.Errorf("arg camera_id is required")
		h.log.Error().Msgf("error: %s", err)
		return nil, err
	}

	cameraId, ok := cameraID.(float64)
	if !ok {
		return nil, fmt.Errorf("camera_id should be int")
	}
	cam, err := h.device.GetCamera(int(cameraId))
	if err != nil {
		h.log.Error().Msgf("get camera error: %s", err)
		return nil, err
	}
	return cam, nil
}

func (h *handler) getStreamManager() *stream.Manager {
	return stream.GetManager(h.device.GetConfig())
}

func (h handler) getStreamEncodeInfo(cam base.Camera, resolution string) *univiewapi.VideoEncodeInfo {
	resolutionId := utils.GetResolutionIdByString(resolution)

	ac, ok := cam.(base.AICamera)
	if !ok {
		return nil
	}
	nvrCli, err := h.device.GetNVRManager().GetNVRClientBySN(ac.GetNvrSN())
	if err != nil {
		return nil
	}
	localStreamSettings, err := nvrCli.GetChannelStreamDetailInfo(ac.GetChannel())
	if err != nil || localStreamSettings == nil {
		return nil
	}
	for _, vsi := range localStreamSettings.VideoStreamInfos {
		if resolutionId == int(vsi.ID) && vsi.MainStreamType == 0 {
			return &vsi.VideoEncodeInfo
		}
	}
	return nil
}

func (h *handler) getStreamInputUri(cam base.Camera, resolution string, startTime, endTime int64, action streamAction) (string, error) {
	if cam == nil {
		return "", errors.New("invalid camera")
	}
	var err error
	var inputUri string
	streamID := 2
	switch utils.Resolution(resolution) {
	case utils.HD:
		inputUri = cam.GetHdUri()
		streamID = 1
	case utils.SD:
		// only support livestreaming
		if startTime > 0 {
			inputUri = cam.GetUri()
		} else {
			inputUri = cam.GetSdUri()
		}
	default:
		inputUri = cam.GetUri()
	}

	// this is a playback video
	if startTime > 0 {
		if endTime == 0 {
			endTime = startTime + 3600 // default duration: 1h.
		}
		switch cam.GetBrand() {
		case utils.Sunell:
		case utils.Uniview:
			aiCam := cam.(base.AICamera)
			inputUri, err = aiCam.GetPlaybackUrl(inputUri, startTime, endTime)
			if err == nil && cam.GetBrand() == utils.Uniview {
				unvCam := aiCam.(*uniview.BaseUniviewCamera)
				err = unvCam.NvrWriteCacheToDisk(unvCam.GetChannel(), streamID, startTime, endTime)
			}
		default:
			err = errors.New("does not support this brand yet")
		}
	}
	if err != nil {
		return "", err
	}
	u, err := url.Parse(inputUri)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" || u.Path == "" {
		return "", errors.New("invalid input stream uri")
	}
	if u.User == nil && cam.GetUserName() != "" && cam.GetPassword() != "" {
		u.User = url.UserPassword(cam.GetUserName(), cam.GetPassword())
	}

	q := u.Query()
	// used to add stream param to deliver message.
	if (action.Action == string(stream.Speed) || action.Action == string(stream.Scale)) && action.Param > 0 {
		q.Add(action.Action, fmt.Sprintf("%f", action.Param))
	}
	q.Add(utils.HeaderIsLive, strconv.FormatBool(startTime == 0))
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (h *handler) getStreamOutputUri(streamType string, token streamRequestToken) string {
	switch stream.OutputType(streamType) {
	case stream.HlsType:
		return token.Uri
	case stream.DashType, stream.WebRTCType:
		return token.OutputUri
	default:
		return token.Uri
	}
}

func (h *handler) getStreamType(streamType string) string {
	switch stream.OutputType(streamType) {
	case stream.HlsType, stream.DashType, stream.WebRTCType, stream.RTSPType:
		return streamType
	default:
		return stream.HlsType.String()
	}
}

func (h *handler) getRecords(msg websocket.Message) ([]byte, error) {
	args, err := json.Marshal(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), nil
	}
	req := &getRecordsReq{}
	if err := json.Unmarshal(args, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	cam, err := h.device.GetCamera(req.CameraID)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	if cam.GetBrand() == utils.Uniview {
		channel := cam.(*uniview.BaseUniviewCamera).GetChannel()
		nvrSN := cam.(*uniview.BaseUniviewCamera).GetNvrSN()
		retRecords := getRecordsRet{}
		err := h.device.GetAllRecords(nvrSN, channel, req.Begin, req.End, &retRecords)
		if err != nil {
			return msg.ReplyMessage(err).Marshal(), err
		}
		return msg.ReplyMessage(&retRecords).Marshal(), nil
	}
	return nil, err
}

func (h *handler) getRecordsDaily(msg websocket.Message) ([]byte, error) {
	args, err := json.Marshal(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), nil
	}
	req := &getDailyRecordsReq{}
	if err := json.Unmarshal(args, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	cam, err := h.device.GetCamera(req.CameraID)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	channel := cam.(*uniview.BaseUniviewCamera).GetChannel()
	nvrSN := cam.(*uniview.BaseUniviewCamera).GetNvrSN()
	retRecords := getDailyRecordsRet{}
	err = h.device.GetRecordsDaily(nvrSN, channel, uint32(req.Year), uint32(req.Month), &retRecords)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	return msg.ReplyMessage(&retRecords).Marshal(), nil
}

type sdInfos struct {
	NvrSN      string   `json:"nvr_sn"`
	IsReady    bool     `json:"is_ready"`
	SdInfoList []sdInfo `json:"sd_info_list"`
}

type sdInfo struct {
	RemainCapacity int    `json:"remain_capacity"` // emaining capacity of the container(MB)
	TotalCapacity  int    `json:"total_capacity"`  // total capacity of the container(MB)
	Manufacturer   string `json:"manufacturer"`    // name of manufacturer
	Status         uint8  `json:"status"`          // status of container
	Property       int    `json:"property"`        // property of hard disk,0: read and write,1: read,2: backup
}

func (h *handler) getNvrSDInfos(msg websocket.Message) ([]byte, error) {
	result := make([]sdInfos, 0)
	nvrSDInfos := h.device.GetNVRManager().GetAllNvrSDInfos()
	if len(nvrSDInfos) == 0 {
		err := fmt.Errorf("no found nvr sd infos")
		return msg.ReplyMessage(err).Marshal(), err
	}

	for sn, v := range nvrSDInfos {
		resInfos := sdInfos{NvrSN: sn, IsReady: v.IsDiskResReady == 1}
		resInfos.SdInfoList = make([]sdInfo, len(v.LocalHDDList))
		for i, hdd := range v.LocalHDDList {
			resInfos.SdInfoList[i] = sdInfo{
				RemainCapacity: hdd.RemainCapacity,
				TotalCapacity:  hdd.TotalCapacity,
				Manufacturer:   hdd.Manufacturer,
				Status:         uint8(hdd.Status),
				Property:       hdd.Property,
			}
		}
		result = append(result, resInfos)
	}
	return msg.ReplyMessage(&result).Marshal(), nil
}

func (h *handler) changeNvrUserInfo(username, password, nvrSN, newPassword string) error {
	nc, err := h.device.GetNVRManager().GetNVRClientBySN(nvrSN)
	if err != nil {
		return err
	}

	rsaInfo, err := nc.GetSystemSecurityRSA()
	if err != nil {
		return err
	}
	// var bigN *big.Int
	bigN := new(big.Int)
	_, ok := bigN.SetString(rsaInfo.RSAPublicKeyN, 16)
	if !ok {
		return fmt.Errorf("bigInt error")
	}

	rsaPublicKeyE, err := strconv.Atoi(rsaInfo.RSAPublicKeyE)
	if err != nil {
		return fmt.Errorf("rsa E fail")
	}
	pub := rsa.PublicKey{
		N: bigN,
		E: rsaPublicKeyE,
	}

	rsaPassword, err := rsa.EncryptPKCS1v15(rand.Reader, &pub, []byte(password))
	if err != nil {
		return fmt.Errorf("old password rsa encrypt error")
	}
	base64Password := base64.StdEncoding.EncodeToString(rsaPassword)

	rsaNewPassword, err := rsa.EncryptPKCS1v15(rand.Reader, &pub, []byte(newPassword))
	if err != nil {
		return fmt.Errorf("new password rsa encrypt error")
	}
	base64NewPassword := base64.StdEncoding.EncodeToString(rsaNewPassword)

	info := &univiewapi.SecurityUserPINInfo{
		UserName:     username,
		CurrentPIN:   base64Password,
		NewPIN:       base64NewPassword,
		RSAPublicKey: rsaInfo,
	}
	return nc.PutSystemSecurityUserPIN(info)
}

func (h *handler) setNvrUserInfo(msg websocket.Message) ([]byte, error) {
	argsMarshalled, err := json.Marshal(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), nil
	}

	req := &setNvrUserInfoReq{}
	if err := json.Unmarshal(argsMarshalled, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	mValidator := validator.New()
	if err := mValidator.Struct(req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	managed, serviceLocked := h.device.GetNVRManager().IsNvrServiceLocked(req.NvrSN)
	if !managed {
		err := fmt.Errorf("reset nvr password failed, nvr not managed, nvr_sn: %s", req.NvrSN)
		return msg.ReplyMessage(err).Marshal(), err
	}
	if !serviceLocked {
		err := fmt.Errorf("reset nvr password failed, please lock the nvr client firstly, nvr_sn: %s", req.NvrSN)
		return msg.ReplyMessage(err).Marshal(), err
	}

	username, password, _, _, err := h.device.GetNVRManager().ReadNVRFromDB(req.NvrSN)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	if username != req.Username || password != req.OldPassword {
		err = fmt.Errorf("username or current password error. nvr sn: %s", req.NvrSN)
		return msg.ReplyMessage(err).Marshal(), err
	}

	err = h.changeNvrUserInfo(username, password, req.NvrSN, req.NewPassword)
	return msg.ReplyMessage(err).Marshal(), err
}

func (h *handler) lockNvrClient(msg websocket.Message) ([]byte, error) {
	argsMarshalled, err := json.Marshal(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), nil
	}

	req := &lockNvrClientReq{}
	if err := json.Unmarshal(argsMarshalled, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	mValidator := validator.New()
	if err := mValidator.Struct(req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	h.device.GetNVRManager().LockNvrClients(req.NvrSN, time.Second*10)
	scheduler.GetScheduler().Suspend(time.Second * 10)
	scheduler.GetScheduler().CleanWaitingQueue()
	return msg.ReplyMessage(nil).Marshal(), nil
}

func (h *handler) activateNvr(msg websocket.Message) ([]byte, error) {
	req := &activateNvr{}
	err := mapstructure.Decode(msg.GetArgs(), req)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	mValidator := validator.New()
	if err := mValidator.Struct(req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	activateClient := univiewapi.NewNoAuthNvrClient(req.IP, h.device.GetNVRManager().GetHttpPort(req.IP),
		univiewapi.WithHttpClientWrap(metrics.HttpClientCounterCollect),
		univiewapi.WithLogger(h.log))
	status, _ := activateClient.SecurityActivateStatus()
	if status == nil {
		err = fmt.Errorf("get nvr activate status error, maybe nvr version not support or nvr stopped")
		return msg.ReplyMessage(err).Marshal(), err
	} else if status.Status == NvrActivated {
		err = fmt.Errorf("nvr have already activated")
		return msg.ReplyMessage(err).Marshal(), err
	}

	rsaEncryptor := pem.GetEncryptor()
	rsaPub := rsaEncryptor.GetDestPublicKey()
	challenge, err := activateClient.SystemSecurityChallenge(&univiewapi.SecurityRSAInfo{
		RSAPublicKeyE: fmt.Sprintf("%x", rsaPub.E),
		RSAPublicKeyN: rsaPub.N.Text(16),
	})
	if err != nil {
		err = fmt.Errorf("get challenge from nvr failed: %s", err.Error())
		return msg.ReplyMessage(err).Marshal(), err
	}
	challengeBase64Decoded, err := base64.StdEncoding.DecodeString(challenge.Key)
	if err != nil {
		err = fmt.Errorf("decode challenge key error: %s", err.Error())
		return msg.ReplyMessage(err).Marshal(), err
	}

	challengeKey, err := rsaEncryptor.DoPrivateDecrypt(challengeBase64Decoded)
	if err != nil {
		err = fmt.Errorf("decrypt challenge failed: %s", err.Error())
		return msg.ReplyMessage(err).Marshal(), err
	}
	passwordEncrypted := aes.AES128ECBZeroPaddingEncrypt(challengeKey[:16], fmt.Sprintf("<%s>", req.Password))
	err = activateClient.SystemSecurityActivate(&univiewapi.SecurityActivateInfo{
		Cipher:  "AES-128",
		Content: base64.StdEncoding.EncodeToString(passwordEncrypted),
	})
	if err != nil {
		err = fmt.Errorf("activate failed: %s", err.Error())
	}
	return msg.ReplyMessage(err).Marshal(), err
}

func (h *handler) getActivateStatus(msg websocket.Message) ([]byte, error) {
	req := &activateStatus{}
	err := mapstructure.Decode(msg.GetArgs(), req)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	mValidator := validator.New()
	if err := mValidator.Struct(req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	activateClient := univiewapi.NewNoAuthNvrClient(req.IP, h.device.GetNVRManager().GetHttpPort(req.IP),
		univiewapi.WithHttpClientWrap(metrics.HttpClientCounterCollect),
		univiewapi.WithLogger(h.log),
		univiewapi.WithHTTPTimeout(time.Second*3))
	status, err := activateClient.SecurityActivateStatus()
	if err != nil {
		err = fmt.Errorf("get nvr activate status error")
		return msg.ReplyMessage(err).Marshal(), err
	} else {
		return msg.ReplyMessage(status).Marshal(), err
	}
}

func (h *handler) setNvrName(msg websocket.Message) ([]byte, error) {
	argsMarshalled, err := json.Marshal(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), nil
	}

	req := &nvrDeviceNameInfo{}
	if err := json.Unmarshal(argsMarshalled, &req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	mValidator := validator.New()
	if err := mValidator.Struct(req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	nc, err := h.device.GetNVRManager().GetNVRClientBySN(req.NvrSN)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	err = nc.PutDeviceName(req.NvrName)
	return msg.ReplyMessage(err).Marshal(), err
}

func (h *handler) setCameraSettingsToNvr(data []*cloud.CameraSettings) (error, []map[string]string) {
	rets := []map[string]string{}
	nvrSystemSnapshotRuleEnable := make(map[string]bool, 0)
	for index := range data {
		rets = append(rets, map[string]string{
			"cloud_event_types":  "not changed",
			"video_capabilities": "not changed",
			"stream_settings":    "not changed",
			"audio_settings":     "not changed",
			"osd_settings":       "not changed",
			"osd_capabilities":   "not changed",
			"audio_input":        "not changed",
			"record_schedule":    "not changed",
		})
		settings := data[index]
		bc, err := h.device.GetCamGroup().GetCameraBySN(settings.CamSN)
		if err != nil {
			return err, rets
		}
		ac, ok := bc.(base.AICamera)
		if !ok || !ac.GetOnline() {
			h.log.Error().Msgf("camera %d is not a aicamera or camera offline", settings.CamID)
			continue
		}
		// Check plugged
		plugged := h.device.GetNVRManager().IsCameraPlugged(ac)
		if !plugged {
			h.log.Error().Msgf("camera %d is not plugged", settings.CamID)
			continue
		}

		nc, err := h.device.GetNVRManager().GetNVRClientBySN(ac.GetNvrSN())
		if err != nil {
			h.log.Error().Msgf("nvr sn %s not found for: %s", ac.GetNvrSN(), err.Error())
			continue
		}

		localSettings := cloud.GetCameraSettingsByID(settings.CamID)
		// StreamSettings
		if settings.StreamSettings != nil {
			if localSettings == nil || !isSameVideoStream(localSettings.StreamSettings, settings.StreamSettings) {
				if err := nc.SetChannelStreamDetailInfo(ac.GetChannel(), &univiewapi.VideoStreamInfos{
					Num:              uint32(len(settings.StreamSettings)),
					VideoStreamInfos: settings.StreamSettings,
				}); err != nil {
					rets[index]["stream_settings"] = err.Error()
				} else {
					rets[index]["stream_settings"] = "ok"
				}
			}
		}

		// VideoSettings
		if settings.AudioSettings != nil {
			if localSettings == nil || !isSameAudioSettings(localSettings.AudioSettings, settings.AudioSettings) {
				if err := nc.SetAudioStatuses(ac.GetChannel(), settings.AudioSettings); err != nil {
					rets[index]["audio_settings"] = err.Error()
				} else {
					rets[index]["audio_settings"] = "ok"
				}
			}
		}

		// OSDSettings
		if settings.OSDSettings != nil {
			if localSettings == nil || !isSameOSDSettings(localSettings.OSDSettings, settings.OSDSettings) {
				// check the camera name
				var cameraNameValid bool
			osdCheck:
				for _, osdContent := range settings.OSDSettings.ContentList {
					for _, contentInfo := range osdContent.ContentInfoList {
						if contentInfo.ContentType == uniview.OsdContentTypeCameraName &&
							len(contentInfo.Value) > 0 {
							cameraNameValid = true
							break osdCheck
						}
					}
				}
				if cameraNameValid {
					if err := nc.SetOSDSettings(ac.GetChannel(), settings.OSDSettings); err != nil {
						rets[index]["osd_settings"] = err.Error()
					} else {
						rets[index]["osd_settings"] = "ok"
					}
				} else {
					rets[index]["osd_settings"] = "camera name invalid"
				}
			}
		}

		// Audio input
		if settings.AudioInput != nil {
			if localSettings == nil || !isSameAudioInput(localSettings.AudioInput, settings.AudioInput) {
				if err := nc.SetChannelMediaAudioInput(ac.GetChannel(), settings.AudioInput); err != nil {
					if err == univiewapi.ErrorInvalidLAPI {
						rets[index]["audio_input"] = "this nvr firmware version not support this feature"
					} else {
						rets[index]["audio_input"] = err.Error()
					}
				} else {
					rets[index]["audio_input"] = "ok"
				}
			}
		}

		// Record Schedule
		if settings.RecordSchedule != nil {
			if localSettings == nil || !isSameRecordSchedule(localSettings.RecordSchedule, settings.RecordSchedule) {
				if err := nc.SetChannelStorageScheduleRecord(ac.GetChannel(), settings.RecordSchedule); err != nil {
					if err == univiewapi.ErrorInvalidLAPI {
						rets[index]["record_schedule"] = "this nvr firmware version not support this feature"
					} else {
						rets[index]["record_schedule"] = err.Error()
					}
				} else {
					rets[index]["record_schedule"] = "ok"
				}
			}
		}

		// EventSettings
		if settings.CloudEventTypes != nil {
			if cloud.HasEventType(ac.GetID(), cloud.MotionStart) {
				nvrSystemSnapshotRuleEnable[ac.GetNvrSN()] = true
			}
		}
	}

	for nvrSn, enable := range nvrSystemSnapshotRuleEnable {
		nc, err := h.device.GetNVRManager().GetNVRClientBySN(nvrSn)
		if err != nil {
			continue
		}
		if err := nc.PutSystemSnapshotRule(enable); err != nil {
			h.log.Error().Msgf("put system snapshot rule error %s", err)
		}
	}
	return nil, rets
}

func (h *handler) refreshCameraSettings(msg websocket.Message) ([]byte, error) {
	// scene 1:  camera settings model changes => notify box by django signal
	// => settings will be all keys here, and website will ignore the return msg.
	// scene 2:  ui change settings => website => box
	// => settings will be not all keys, website will use the return msg, than save camera_settings and return msg to app
	// => than will trigger scene 1.

	data := &cloud.CameraSettingsData{}
	err := mapstructure.Decode(msg.GetArgs(), data)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	mValidator := validator.New()
	if err = mValidator.Struct(data); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	err, rets := h.setCameraSettingsToNvr(data.Data)
	// if settings set to nvr failed, we do not need set to local settings.
	for index := range rets {
		ret := rets[index]
		if ret["stream_settings"] != "ok" {
			data.Data[index].StreamSettings = nil
		}
		if ret["audio_settings"] != "ok" {
			data.Data[index].AudioSettings = nil
		}
		if ret["osd_settings"] != "ok" {
			data.Data[index].OSDSettings = nil
		}
		if ret["audio_input"] != "ok" {
			data.Data[index].AudioInput = nil
		}
		if ret["record_schedule"] != "ok" {
			data.Data[index].RecordSchedule = nil
		}
	}

	// set to local settings
	cloud.SaveCameraSettings(data.Data)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	} else {
		return msg.ReplyMessage(rets).Marshal(), nil
	}
}

func (h *handler) startBackwardAudio(msg websocket.Message) ([]byte, error) {
	var bwar backwardAudioReq
	err := mapstructure.Decode(msg.GetArgs(), &bwar)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	mValidator := validator.New()
	if err = mValidator.Struct(bwar); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	// backward audio is only allowed one audio input to camera.
	if h.getStreamManager().HasStream(bwar.StreamId) {
		return msg.ReplyMessage(websocket.Err{
			Code:           -1,
			DevelopMessage: ErrBackwardAudioOngoing.Error(),
			Message:        ErrBackwardAudioOngoing.Error(),
		}).Marshal(), nil
	}

	cam, err := h.getCameraFromArgs(msg.GetArgs())
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	aiCam, ok := cam.(base.AICamera)
	if !ok {
		return msg.ReplyMessage(ErrInvalidAICamera).Marshal(), nil
	}
	outputUrl := h.getBackwardAudioUrl(aiCam)
	if outputUrl == "" {
		return msg.ReplyMessage(ErrNotBackwardAudioCamera).Marshal(), nil
	}

	inputUrl := bwar.OutputUri

	outputUri, err := h.getStreamManager().StartStream(bwar.StreamId, string(stream.BackwardAudioType), inputUrl, outputUrl)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	h.log.Info().Msgf("pull stream from %s and send to %s", inputUrl, outputUri)
	audioEncodeType := AudioEncodeTypeG711U // default set to mulaw
	if cam.GetBrand() == utils.Uniview {
		audioEncodeType = AudioEncodeTypeG711U
	}

	return msg.ReplyMessage(backwardAudioResp{EncodeInfo: AudioEncodeInfo{
		EncodeType: audioEncodeType,
	}}).Marshal(), err
}

func (h *handler) stopBackwardAudio(msg websocket.Message) ([]byte, error) {
	var bwar backwardAudioHeartReq
	err := mapstructure.Decode(msg.GetArgs(), &bwar)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	mValidator := validator.New()
	if err = mValidator.Struct(bwar); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	// backward audio has not started.
	if !h.getStreamManager().HasStream(bwar.StreamId) {
		return msg.ReplyMessage(websocket.Err{
			Code:           -1,
			DevelopMessage: ErrBackwardAudioNotStarted.Error(),
			Message:        ErrBackwardAudioNotStarted.Error(),
		}).Marshal(), nil
	}

	// stop stream.
	err = h.getStreamManager().StopStream(bwar.StreamId)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	return msg.ReplyMessage(nil).Marshal(), nil
}

func (h *handler) getBackwardAudioUrl(aiCam base.AICamera) string {
	nc, err := h.device.GetNVRManager().GetNVRClientBySN(aiCam.GetNvrSN())
	if err != nil {
		return ""
	}
	channelId := aiCam.GetChannel()
	outputUrl := nc.GetChannelMediaTalk(channelId)
	if outputUrl == "" {
		return ""
	}
	if u, err := url.Parse(outputUrl); err == nil {
		if u.Scheme == "" || u.Host == "" || u.Path == "" {
			return ""
		}
		if u.User == nil && aiCam.GetUserName() != "" && aiCam.GetPassword() != "" {
			u.User = url.UserPassword(aiCam.GetUserName(), aiCam.GetPassword())
		}
		return u.String()
	}
	return ""
}
func (h *handler) heartBackwardAudio(msg websocket.Message) ([]byte, error) {
	var bwar backwardAudioHeartReq
	err := mapstructure.Decode(msg.GetArgs(), &bwar)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	mValidator := validator.New()
	if err = mValidator.Struct(bwar); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	// backward audio has not started.
	if !h.getStreamManager().HasStream(bwar.StreamId) {
		return msg.ReplyMessage(websocket.Err{
			Code:           -1,
			DevelopMessage: ErrBackwardAudioNotStarted.Error(),
			Message:        ErrBackwardAudioNotStarted.Error(),
		}).Marshal(), nil
	}

	// judge if stream is alive.
	if h.getStreamManager().IsStreamStopped(bwar.StreamId) {
		return msg.ReplyMessage(websocket.Err{
			Code:           -2,
			DevelopMessage: ErrBackwardAudioHasStopped.Error(),
			Message:        ErrBackwardAudioHasStopped.Error(),
		}).Marshal(), nil
	}

	return msg.ReplyMessage(nil).Marshal(), nil
}

func (h *handler) handleBoxSetting(msg websocket.Message) ([]byte, error) {
	var bsr UpdateBoxSettingReq
	err := mapstructure.Decode(msg.GetArgs(), &bsr)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	mValidator := validator.New()
	if err = mValidator.Struct(bsr); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	//if bsr.MaxPlaybackSize > 0 {
	//	_ = h.device.GetConfig().SetMaxPlaybackSize(bsr.MaxPlaybackSize)
	//}
	// TODO currently, we are using max playback to control clip size, at next, we
	// should use a new field like max clip size to control this. and playback control itself.
	if bsr.MaxPlaybackSize > 0 {
		_ = h.device.GetConfig().SetMaxClipSize(bsr.MaxPlaybackSize)
	}
	if bsr.MaxLivestreamSize > 0 {
		_ = h.device.GetConfig().SetMaxLivestreamSize(bsr.MaxLivestreamSize)
	}
	if bsr.MaxPlaybackSize > 0 || bsr.MaxLivestreamSize > 0 {
		scheduler.GetScheduler().RefreshSchedulerSize(h.device.GetConfig().GetStreamConfig())
	}

	// can't set 0
	if bsr.EventSavedHours > 0 {
		_ = h.device.GetConfig().SetEventSavedHours(bsr.EventSavedHours)
	}
	if bsr.EventMaxRetry > 0 {
		_ = h.device.GetConfig().SetEventRetryCount(bsr.EventMaxRetry)
	}

	bsr = UpdateBoxSettingReq{
		MaxLivestreamSize: h.device.GetConfig().GetStreamConfig().MaxLivestreamSize,
		MaxPlaybackSize:   h.device.GetConfig().GetStreamConfig().MaxClipSize,
		EventMaxRetry:     int(h.device.GetConfig().GetEventRetryCount()),
		EventSavedHours:   h.device.GetConfig().GetEventSavedHours(),
	}
	return msg.ReplyMessage(bsr).Marshal(), nil
}

func (h *handler) isIotDeviceAllFound(devices []IotDevice, from, to uint32, ips, macs []string) bool {
	foundIps := []string{}
	foundMacs := []string{}
	for _, dev := range devices {
		foundIps = append(foundIps, dev.IpAddress)
		foundMacs = append(foundMacs, dev.MacAddress)
	}
	// check all
	if from == 0 && to == 0 && len(ips) == 0 && len(macs) == 0 {
		return false
	}
	// check range filter
	if from > 0 || to > 0 {
		for tmpIp := from; tmpIp <= to; tmpIp++ {
			if !utils.ContainsString(foundIps, utils.IP(tmpIp).String()) {
				return false
			}
		}
	}
	// check ip filter
	if len(ips) > 0 {
		for _, tmpIp := range ips {
			if !utils.ContainsString(foundIps, tmpIp) {
				return false
			}
		}
	}
	// check mac filter
	if len(foundMacs) > 0 {
		for _, tmpMac := range macs {
			if len(tmpMac) != 17 {
				// Prefix
				return false
			}
			if !utils.ContainsString(foundMacs, tmpMac) {
				return false
			}
		}
	}
	return true
}

func (h *handler) iotDiscover(msg websocket.Message) ([]byte, error) {
	// Do arp search
	if !h.device.GetConfig().GetArpEnable() {
		return msg.ReplyMessage(nil).Marshal(), errors.New("arp not enable")
	}

	var req IotDeviceDiscoverReq
	err := mapstructure.Decode(msg.GetArgs(), &req)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	mValidator := validator.New()
	if err = mValidator.Struct(req); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	isRange := false
	var ipFrom, ipTo uint32
	if len(req.Filter.IPRange.From) > 0 && len(req.Filter.IPRange.To) > 0 {
		isRange = true
		ipFrom = uint32(utils.ParseIPString(req.Filter.IPRange.From))
		ipTo = uint32(utils.ParseIPString(req.Filter.IPRange.To))
		if ipFrom > ipTo {
			ipFrom, ipTo = ipTo, ipFrom
		}
	}
	targetIpMap := make(map[string]string)
	for _, ip := range req.Filter.IPAddress {
		targetIpMap[ip] = ip
	}
	withIpFilter := false
	if isRange || len(req.Filter.IPAddress) > 0 {
		withIpFilter = true
		go h.device.GetArpSearcher().SearchWithIpFilter(ipFrom, ipTo, req.Filter.IPAddress)
	} else {
		go h.device.GetArpSearcher().SearchAll()
	}

	macPrefixLst := []string{}
	for _, mac := range req.Filter.MacAddress {
		macPrefixLst = append(macPrefixLst, strings.ToLower(mac))
	}

	// Do onvif search
	onvifDevices := []onvif.Device{}
	go func() {
		onvifDevices = h.device.GetSearcher().StartSearch()
	}()

	devices := []IotDevice{}
	resp := IotDiscoverResp{
		Status:  "scanning",
		Devices: devices,
	}
	go func(msg websocket.Message) {
		// Get devices for every 3 seconds,
		reportInterval := h.device.GetConfig().GetArpReportInterval()
		reportTimes := int(h.device.GetConfig().GetArpReportTimes())
		for i := 0; i < reportTimes; i++ {
			time.Sleep(time.Duration(reportInterval) * time.Second)
			arpDeviceMap := h.device.GetArpSearcher().GetDevices()
			h.log.Debug().Msgf("iot discover arp devices: %+v", arpDeviceMap)
			h.log.Debug().Msgf("iot discover onvif devices: %+v", onvifDevices)
			// Filter onvif device
			onvifDeviceMapFiltered := make(map[string]onvif.Device)
			for index := range onvifDevices {
				ip, _ := utils.ParseXAddr(onvifDevices[index].Params.Xaddr)
				mac := onvifDevices[index].Info.MACAddress
				mac = strings.ToLower(mac)
				if len(macPrefixLst) > 0 && !utils.ContainsStringPrefixes(macPrefixLst, mac) {
					continue
				}
				if withIpFilter {
					_, ok := targetIpMap[ip]
					if !ok && !isRange {
						continue
					}
					if !ok && isRange && (uint32(utils.ParseIPString(ip)) < ipFrom || uint32(utils.ParseIPString(ip)) > ipTo) {
						continue
					}
				}
				onvifDeviceMapFiltered[mac] = onvifDevices[index]
			}

			// Filter arp device
			arpDeviceMapFiltered := make(map[string]arp.Device)
			for mac, dev := range arpDeviceMap {
				ip := dev.IP
				mac = strings.ToLower(mac)
				if len(macPrefixLst) > 0 && !utils.ContainsStringPrefixes(macPrefixLst, mac) {
					continue
				}
				if withIpFilter {
					_, ok := targetIpMap[ip]
					if !ok && !isRange {
						continue
					}
					if !ok && isRange && (uint32(utils.ParseIPString(ip)) < ipFrom || uint32(utils.ParseIPString(ip)) > ipTo) {
						continue
					}
				}
				arpDeviceMapFiltered[mac] = dev
			}

			h.log.Debug().Msgf("iot discover arp devices filtered: %+v", arpDeviceMapFiltered)
			h.log.Debug().Msgf("iot discover onvif devices filtered: %+v", onvifDeviceMapFiltered)

			// Merge arp devices and onvif devices
			resp.Devices = h.mergeDevices(arpDeviceMapFiltered, onvifDeviceMapFiltered)
			if h.isIotDeviceAllFound(resp.Devices, ipFrom, ipTo, req.Filter.IPAddress, req.Filter.MacAddress) && len(onvifDevices) > 0 {
				break
			}
			h.device.WsClient().Send(msg.ReplyMessage(resp).Marshal())
		}
		time.Sleep(time.Duration(reportInterval) * time.Second)
		resp.Status = "done"
		h.device.WsClient().Send(msg.ReplyMessage(resp).Marshal())
	}(msg)

	return msg.ReplyMessage(resp).Marshal(), nil
}

func (h *handler) mergeDevices(arpDeviceMap map[string]arp.Device, onvifDeviceMap map[string]onvif.Device) []IotDevice {
	ret := []IotDevice{}
	macProcessed := make(map[string]interface{})
	for mac, arpDevice := range arpDeviceMap {
		macProcessed[mac] = mac
		var sn, deviceModel, manufacturer, firmwareVersion string
		if onvifDevice, ok := onvifDeviceMap[mac]; ok {
			ip, _ := utils.ParseXAddr(onvifDevice.Params.Xaddr)
			if ip != arpDevice.IP {
				// Onvif device & arp device not matched
				h.log.Debug().Msgf("device info is dirty, mac: %s, onvif_ip: %s, arp_ip: %s",
					mac, onvifDevice.Params.Xaddr, arpDevice.IP)
				continue
			}
			sn = onvifDevice.Info.SerialNumber
			deviceModel = onvifDevice.Info.Model
			manufacturer = onvifDevice.Info.Manufacturer
			firmwareVersion = onvifDevice.Info.FirmwareVersion
		}
		ret = append(ret, IotDevice{
			MacAddress:      mac,
			IpAddress:       arpDevice.IP,
			SerialNumber:    sn,
			DeviceModel:     deviceModel,
			Manufacturer:    manufacturer,
			FirmwareVersion: firmwareVersion,
		})
	}
	for mac, onvifDevice := range onvifDeviceMap {
		if _, ok := macProcessed[mac]; !ok {
			ip, _ := utils.ParseXAddr(onvifDevice.Params.Xaddr)
			ret = append(ret, IotDevice{
				MacAddress:      mac,
				IpAddress:       ip,
				SerialNumber:    onvifDevice.Info.SerialNumber,
				DeviceModel:     onvifDevice.Info.Model,
				Manufacturer:    onvifDevice.Info.Manufacturer,
				FirmwareVersion: onvifDevice.Info.FirmwareVersion,
			})
		}
	}
	return ret
}

// Send HTTP request with digest auth to AXIS Speaker. Only supports Get
func sendRequestToAXISSpeaker(url, username, password string) (*http.Response, error) {
	client := &http.Client{
		Transport: &digest.Transport{
			Username: username,
			Password: password,
		},
	}
	res, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	// check status code
	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusUnauthorized {
			return nil, ErrHttpAuthFailed
		}
		return nil, fmt.Errorf("http status code: %d", res.StatusCode)
	}
	return res, err
}

func (h *handler) iotPlayAudioClip(msg websocket.Message) ([]byte, error) {
	// Bind and validate req
	var req actIotPlayAudioClipReq
	err := mapstructure.Decode(msg.GetArgs(), &req)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	if err = req.Validate(); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	// Send http req to device
	url := fmt.Sprintf(axisPlayClipUrl, req.IPAddress, req.MediaID, req.Volume)
	_, err = sendRequestToAXISSpeaker(url, req.Username, req.Password)
	resp := actIotPlayAudioClipResp{
		Status: "ok",
		Msg:    "",
	}
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	return msg.ReplyMessage(resp).Marshal(), nil
}

func (h *handler) iotStopAudioClip(msg websocket.Message) ([]byte, error) {
	// Bind and validate req
	var req actIotStopAudioReq
	err := mapstructure.Decode(msg.GetArgs(), &req)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	if err = req.Validate(); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	// Send http request to device
	url := fmt.Sprintf(axisStopClipUrl, req.IPAddress)
	_, err = sendRequestToAXISSpeaker(url, req.Username, req.Password)
	resp := actIotStopAudioResp{
		Status: "ok",
		Msg:    "",
	}
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	return msg.ReplyMessage(resp).Marshal(), nil
}

func (h *handler) iotValidateSpeaker(msg websocket.Message) ([]byte, error) {
	// Bind and validate req
	var req actIotValidateSpeakerReq
	err := mapstructure.Decode(msg.GetArgs(), &req)
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	if err = req.Validate(); err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}

	// Send http request to speaker
	url := fmt.Sprintf(axisListClipUrl, req.IPAddress)
	_, err = sendRequestToAXISSpeaker(url, req.Username, req.Password)
	resp := actIotValidateSpeakerResp{
		Status: "ok",
		Msg:    "",
	}
	if err != nil {
		return msg.ReplyMessage(err).Marshal(), err
	}
	return msg.ReplyMessage(resp).Marshal(), nil
}
