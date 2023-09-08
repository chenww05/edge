package box

import (
	"errors"

	"github.com/go-playground/validator/v10"

	"github.com/example/minibox/utils"
)

type srsApiReq struct {
	Sdp       string `json:"sdp" validate:"required"`
	ClientIp  string `json:"clientip" validate:"required"`
	StreamUrl string `json:"streamurl" validate:"required"`
}
type srsApiRsp struct {
	Code      int    `json:"code" validate:"required"`
	Server    string `json:"server" validate:"required"`
	Sdp       string `json:"sdp" validate:"required"`
	Sessionid string `json:"sessionid" validate:"required"`
}

type sdpTransportReq struct {
	Id  string          `json:"id" validate:"required"`
	Act string          `json:"act" validate:"required"`
	Arg sdpTransportArg `json:"arg" validate:"required"`
}

type sdpTransportArg struct {
	CameraId int    `json:"camera_id" validate:"required,number"`
	StreamId string `json:"stream_id" validate:"required"`
	SdpLocal string `json:"sdp_local" validate:"required"`
}

type sdpResponse struct {
	CameraId  int    `json:"camera_id"`
	StreamId  string `json:"stream_id"`
	SdpRemote string `json:"sdp_remote"`
}

type streamRequestParam struct {
	Id  string           `json:"id" validate:"required"`
	Act string           `json:"act" validate:"required"`
	Arg streamRequestArg `json:"arg" validate:"required"`
}

type singleStream struct {
	CameraId     int64              `json:"camera_id" mapstructure:"camera_id"`
	Resolution   string             `json:"resolution" mapstructure:"resolution"`
	StartTime    int64              `json:"start_time" mapstructure:"start_time"`
	EndTime      int64              `json:"end_time" mapstructure:"end_time"`
	StreamType   string             `json:"stream_type" mapstructure:"stream_type"`
	StreamId     string             `json:"stream_id" mapstructure:"stream_id"`
	StreamMode   string             `json:"stream_mode" mapstructure:"stream_mode"`
	StreamAction streamAction       `json:"stream_action" mapstructure:"stream_action"`
	Token        streamRequestToken `json:"token"`
	Err          struct {
		Code    int    `json:"code" mapstructure:"code"`
		Message string `json:"message" mapstructure:"message"`
	} `json:"err" mapstructure:"err"`
}

type StreamInfo struct {
	Param []singleStream `json:"param" mapstructure:"param"`
}

type streamMultiRequestParam struct {
	Id  string     `json:"id" validate:"required"`
	Act string     `json:"act" validate:"required"`
	Arg StreamInfo `json:"arg" validate:"required"`
}

type streamRequestArg struct {
	CameraId     int                `json:"camera_id" validate:"required,number"`
	Token        streamRequestToken `json:"token" validate:"required"`
	Err          err                `json:"err" validate:"required"`
	StreamParam  streamParam        `json:"stream_param"`
	StreamAction streamAction       `json:"stream_action" mapstructure:"stream_action"`
}

type streamRequestToken struct {
	Uri       string `json:"uri"`
	BaseUri   string `json:"base_uri"`
	OutputUri string `json:"output_uri" validate:"omitempty,url"`
}

type err struct {
	Code    int    `json:"code" mapstructure:"code"`
	Message string `json:"message" mapstructure:"message"`
}

type streamParam struct {
	StreamId   string `json:"stream_id"`
	StreamType string `json:"stream_type"`
	StartTime  int64  `json:"start_time"`
	EndTime    int64  `json:"end_time"`
	Resolution string `json:"resolution"`
}

type streamAction struct {
	Action string  `json:"action" mapstructure:"action"`
	Param  float32 `json:"param" mapstructure:"param"`
}

type streamResponse struct {
	BaseUri    string      `json:"base_uri"`
	StreamType string      `json:"stream_type"`
	StreamId   string      `json:"stream_id"`
	EncodeInfo interface{} `json:"encode_info,omitempty"`
	OutputUri  string      `json:"uri"`
	RtmpUri    string      `json:"rtmp_uri"`
}

type streamInfo struct {
	CameraId   int64       `json:"camera_id"`
	BaseUri    string      `json:"base_uri"`
	StreamType string      `json:"stream_type"`
	StreamId   string      `json:"stream_id"`
	EncodeInfo interface{} `json:"encode_info,omitempty"`
	OutputUri  string      `json:"output_uri"`
	RtmpUri    string      `json:"rtmp_uri"`
	Resolution string      `json:"resolution"`
	StartTime  int64       `json:"start_time"`
	Err        struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"err"`
}

type multistreamResponse struct {
	Param []*streamInfo `json:"param" mapstructure:"param"`
}

type validateCamReq struct {
	BoxId    string `json:"box_id"`
	Uri      string `json:"uri"`
	HdUri    string `json:"hd_uri"`
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type validateCamRet struct {
	MacAddress string       `json:"mac_address"`
	Snapshot   utils.S3File `json:"snapshot"`
	SDUri      string       `json:"sd_uri"`
	Uri        string       `json:"uri"`
	HDUri      string       `json:"hd_uri"`
	SerialNo   string       `json:"serial_no"`
}

type validateDvrReq struct {
	Host              string `json:"host"`
	BoxId             string `json:"box_id"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	MacAddress        string `json:"mac_address"`
	WithChannelStatus bool   `json:"with_channel_status"`
}

type getRecordsReq struct {
	CameraID int   `json:"camera_id"`
	Begin    int64 `json:"begin"`
	End      int64 `json:"end"`
}

type record struct {
	Type  uint32 `json:"types"`
	Begin int64  `json:"begin"`
	End   int64  `json:"end"`
}

type getRecordsRet struct {
	Num     uint32   `json:"nums"`
	Records []record `json:"records"`
}

type validateDvrRet struct {
	Cameras []validateDvrCameraRet `json:"cameras"`
}

type validateDvrCameraRet struct {
	ChannelId     uint32        `json:"channel_id"`
	ChannelName   string        `json:"channel_name"`
	ChannelStatus uint32        `json:"channel_status"`
	MacAddress    string        `json:"mac_address"`
	NvrSN         string        `json:"nvr_sn"`
	Snapshot      *utils.S3File `json:"snapshot,omitempty"`
	SDUri         string        `json:"sd_uri"`
	Uri           string        `json:"uri"`
	HDUri         string        `json:"hd_uri"`
	SerialNo      string        `json:"serial_no"`
	Brand         string        `json:"brand"`
	Manufacturer  string        `json:"manufacturer"`
	Model         string        `json:"model"`
}

type device struct {
	Host            string `json:"host"`
	MacAddress      string `json:"mac_address"`
	StreamUrl       string `json:"stream_url"`
	Manufacturer    string `json:"manufacturer"` // brand
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmware_version"`
	SerialNo        string `json:"serial_no"`
	HardwareId      string `json:"hardware_id"`
	ActivateStatus  string `json:"activate_status,omitempty"`
}

type searchDevice struct {
	Devices []device `json:"devices"`
}

type ptzCtrlMsg struct {
	CameraID int    `json:"camera_id"`
	Command  string `json:"cmd"`
	HSpeed   int    `json:"h_speed"`
	VSpeed   int    `json:"v_speed"`
}

type ptzPreset struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type getPtzPresetsReq struct {
	CameraID int `json:"camera_id"`
}

type getPtzPresetsRet struct {
	Num     int         `json:"num"`
	Presets []ptzPreset `json:"presets"`
}

type setPtzPresetReq struct {
	CameraID int       `json:"camera_id"`
	Preset   ptzPreset `json:"preset"`
}

type goToPtzPresetReq struct {
	CameraID int    `json:"camera_id"`
	PresetID uint32 `json:"preset_id"`
}

type streamActionReq struct {
	StreamId string  `json:"stream_id" validate:"required" mapstructure:"stream_id"`
	Action   string  `json:"action" validate:"required" mapstructure:"action"`
	Param    float32 `json:"param" mapstructure:"param"`
}

type streamActionResp struct {
	StreamId     string `json:"stream_id"`
	StreamStatus string `json:"stream_status"`
}

type streamSettingsReq struct {
	CameraID     int64 `json:"camera_id" validate:"required" mapstructure:"camera_id"`
	Audio1Enable int   `json:"audio1_enable" mapstructure:"audio1_enable"`
	Audio2Enable int   `json:"audio2_enable" mapstructure:"audio2_enable"`
	Audio3Enable int   `json:"audio3_enable" mapstructure:"audio3_enable"`
}

type imageEnhanceSetReq struct {
	CameraId      int `json:"camera_id" mapstructure:"camera_id"`
	Brightness    int `json:"brightness" mapstructure:"brightness"`
	Contrast      int `json:"contrast" mapstructure:"contrast"`
	Saturation    int `json:"saturation" mapstructure:"saturation"`
	Sharpness     int `json:"sharpness" mapstructure:"sharpness"`
	ImageRotation int `json:"image_rotation" mapstructure:"image_rotation"`
	DNoiseReduce  int `json:"2D_noise_reduce" mapstructure:"2D_noise_reduce"`
	ImageOffset   int `json:"image_offset" mapstructure:"image_offset"`
}

type imageEnhanceGetReq struct {
	CameraId int `json:"camera_id" mapstructure:"camera_id"`
}

type imageEnhanceGetResp struct {
	Brightness    int `json:"brightness"`
	Contrast      int `json:"contrast"`
	Saturation    int `json:"saturation"`
	Sharpness     int `json:"sharpness"`
	ImageRotation int `json:"image_rotation"`
	DNoiseReduce  int `json:"2D_noise_reduce"`
	ImageOffset   int `json:"image_offset"`
}

type lockNvrClientReq struct {
	NvrSN string `json:"nvr_sn" validate:"required" mapstructure:"nvr_sn"`
}

type setNvrUserInfoReq struct {
	NvrSN       string `json:"nvr_sn" validate:"required" mapstructure:"nvr_sn"`
	Username    string `json:"username" validate:"required" mapstructure:"username"`
	OldPassword string `json:"old_password" validate:"required" mapstructure:"old_password"`
	NewPassword string `json:"new_password" validate:"required" mapstructure:"new_password"`
}

type activateNvr struct {
	IP       string `json:"ip" validate:"required" mapstructure:"ip"`
	Password string `json:"password" validate:"required" mapstructure:"password"`
}

type activateStatus struct {
	IP string `json:"ip" validate:"required" mapstructure:"ip"`
}

type nvrDeviceNameInfo struct {
	NvrSN   string `json:"nvr_sn" validate:"required" mapstructure:"nvr_sn"`
	NvrName string `json:"nvr_name" validate:"required" mapstructure:"nvr_name"`
}

type backwardAudioHeartReq struct {
	CameraId int    `json:"camera_id" validate:"required" mapstructure:"camera_id"`
	StreamId string `json:"stream_id" validate:"required" mapstructure:"stream_id"`
}

type AudioDeviceType string

var (
	AudioDTCamera  AudioDeviceType = "camera"
	AudioDTSpeaker AudioDeviceType = "speaker"
)

type generalStartBackwardAudioReq struct {
	DeviceId        int64           `json:"device_id" mapstructure:"device_id"`
	DeviceType      AudioDeviceType `json:"device_type" mapstructure:"device_type"`
	StreamId        string          `json:"stream_id" mapstructure:"stream_id"`
	OutputUri       string          `json:"output_uri" mapstructure:"output_uri"`
	DeviceUsername  string          `json:"device_username" mapstructure:"device_username"`
	DevicePassword  string          `json:"device_password" mapstructure:"device_password"`
	DeviceStreamUri string          `json:"device_stream_uri" mapstructure:"device_stream_uri"`
}

func (req *generalStartBackwardAudioReq) Validate() error {
	if req.StreamId == "" || req.OutputUri == "" {
		return errors.New("stream_id, output_uri can not be empty")
	}
	if req.DeviceType == AudioDTSpeaker {
		if req.DeviceUsername == "" || req.DevicePassword == "" {
			return errors.New("username or password is empty")
		}
		if req.DeviceStreamUri == "" {
			return errors.New("device stream_uri is empty")
		}
	}
	return nil
}

type generalBackwardAudioReq struct {
	DeviceId   int64           `json:"device_id" mapstructure:"device_id"`
	DeviceType AudioDeviceType `json:"device_type" mapstructure:"device_type"`
	StreamId   string          `json:"stream_id" validate:"required" mapstructure:"stream_id"`
}

type backwardAudioReq struct {
	CameraId  int    `json:"camera_id" validate:"required" mapstructure:"camera_id"`
	StreamId  string `json:"stream_id" validate:"required" mapstructure:"stream_id"`
	OutputUri string `json:"output_uri" validate:"required" mapstructure:"output_uri"`
}

type backwardAudioResp struct {
	EncodeInfo AudioEncodeInfo `json:"encode_info"`
}

type AudioEncodeInfo struct {
	EncodeType string `json:"encode_type"`
}

type UpdateBoxSettingReq struct {
	MaxLivestreamSize int `json:"max_livestream_size" mapstructure:"max_livestream_size"`
	MaxPlaybackSize   int `json:"max_playback_size" mapstructure:"max_playback_size"`
	EventSavedHours   int `json:"event_saved_hours"  mapstructure:"event_saved_hours"`
	EventMaxRetry     int `json:"event_max_retry"  mapstructure:"event_max_retry"`
}

type getDailyRecordsReq struct {
	CameraID int `json:"camera_id"`
	Year     int `json:"year"`
	Month    int `json:"month"`
}

type getDailyRecordsRet struct {
	Num      int   `json:"nums"`
	Statuses []int `json:"daily_statuses"`
}

type IotDeviceDiscoverReqFilter struct {
	MacAddress []string                    `json:"mac_address" mapstructure:"mac_address"`
	IPAddress  []string                    `json:"ip_address" mapstructure:"ip_address"`
	IPRange    IotDeviceDiscoverReqIPRange `json:"ip_range" mapstructure:"ip_range"`
}

type IotDeviceDiscoverReqIPRange struct {
	From string `json:"from" mapstructure:"from"`
	To   string `json:"to" mapstructure:"to"`
}

type IotDeviceDiscoverReq struct {
	BoxId  string                     `json:"box_id" mapstructure:"box_id"`
	Filter IotDeviceDiscoverReqFilter `json:"filter" mapstructure:"filter"`
}

type IotDevice struct {
	MacAddress      string `json:"mac_address"`
	IpAddress       string `json:"ip_address"`
	SerialNumber    string `json:"serial_number"`
	DeviceModel     string `json:"device_model"`
	Manufacturer    string `json:"manufacturer"`
	FirmwareVersion string `json:"firmware_version"`
}

type IotDiscoverResp struct {
	Status  string      `json:"status"`
	Devices []IotDevice `json:"devices"`
}

type actIotPlayAudioClipReq struct {
	BoxID     string `json:"box_id" mapstructure:"box_id" validate:"required"`
	DeviceID  int64  `json:"device_id" mapstructure:"device_id" validate:"required"`
	IPAddress string `json:"ip_address" mapstructure:"ip_address" validate:"required"`
	Username  string `json:"username" mapstructure:"username" validate:"required"`
	Password  string `json:"password" mapstructure:"password" validate:"required"`
	MediaID   int64  `json:"media_id" mapstructure:"media_id" validate:"required"`
	Volume    int64  `json:"volume" mapstructure:"volume" validate:"required"`
}

func (req *actIotPlayAudioClipReq) Validate() error {
	return validator.New().Struct(req)
}

type actIotPlayAudioClipResp struct {
	Status string `json:"status" mapstructure:"status"`
	Msg    string `json:"msg" mapstructure:"msg"`
}

type actIotStopAudioReq struct {
	BoxID     string `json:"box_id" mapstructure:"box_id" validate:"required"`
	DeviceID  int64  `json:"device_id" mapstructure:"device_id" validate:"required"`
	IPAddress string `json:"ip_address" mapstructure:"ip_address" validate:"required"`
	Username  string `json:"username" mapstructure:"username" validate:"required"`
	Password  string `json:"password" mapstructure:"password" validate:"required"`
}

func (req *actIotStopAudioReq) Validate() error {
	return validator.New().Struct(req)
}

type actIotStopAudioResp struct {
	Status string `json:"status" mapstructure:"status"`
	Msg    string `json:"msg" mapstructure:"msg"`
}

type actIotValidateSpeakerReq struct {
	BoxID     string `json:"box_id" mapstructure:"box_id" validate:"required"`
	IPAddress string `json:"ip_address" mapstructure:"ip_address" validate:"required"`
	Username  string `json:"username" mapstructure:"username" validate:"required"`
	Password  string `json:"password" mapstructure:"password" validate:"required"`
}

func (req *actIotValidateSpeakerReq) Validate() error {
	return validator.New().Struct(req)
}

type actIotValidateSpeakerResp struct {
	Status string `json:"status" mapstructure:"status"`
	Msg    string `json:"msg" mapstructure:"msg"`
}
