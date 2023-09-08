package apis

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"time"

	"github.com/codegangsta/inject"
	"github.com/gin-gonic/gin"
	"github.com/olekukonko/tablewriter"
	"github.com/rs/zerolog"
	"github.com/example/minibox/box"
	"github.com/example/minibox/cloud"
	"github.com/example/onvif"
	http2 "github.com/example/turing-common/http"
	"github.com/example/turing-common/log"
)

const DayFormat = "2006-01-02"

// for local ui
type DumpAPI struct {
	Box box.Box `inject:"box"`

	logger zerolog.Logger
}

func RegisterDumpAPI(injector inject.Injector, router *gin.Engine) {
	logger := log.Logger("debug_api")
	api := &DumpAPI{
		logger: logger,
	}
	if err := injector.Apply(api); err != nil {
		logger.Fatal().Err(err).Msg("Failed to init debug api.")
	}
	http2.RegisterGinGroupHandler(&router.RouterGroup, api)
}

func (d *DumpAPI) BaseURL() string {
	return "api/dump"
}

func (d *DumpAPI) DumpMiddleware(c *gin.Context) {
	key := c.Query("key")
	nowDay := time.Now().UTC().Format(DayFormat)
	nowKey := fmt.Sprintf("%x", md5.Sum([]byte(
		fmt.Sprintf("%s+%s", d.Box.GetBoxId(), nowDay),
	)))
	if key != nowKey {
		c.Abort()
	}
}

func (d *DumpAPI) Middlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{d.DumpMiddleware}
}

func (d *DumpAPI) Register(group *gin.RouterGroup) {
	// for ppl event
	group.GET("cameras", d.Cameras)
	group.GET("t_cameras", d.TCameras)
	group.GET("camera_settings", d.CameraSettingss)
	group.GET("t_camera_settings", d.TCameraSettingss)
	group.GET("nvr", d.Nvr)
	group.GET("t_nvr", d.TNvr)
	group.GET("nvr_managed", d.NvrManaged)
	group.GET("t_nvr_managed", d.TNvrManaged)
	group.GET("archive", d.ArchiveSetting)
	group.GET("t_archive", d.TArchiveSetting)
	group.GET("cloud_nvr", d.CloudNvr)
	group.GET("t_cloud_nvr", d.TCloudNvr)
}

type CameraStruct struct {
	ID string `json:"camera_id"`
	IP string `json:"ip"`
	SN string `json:"device_sn"`
	//Version      string `json:"version"`
	Name         string `json:"name"`
	Online       string `json:"online"`
	Brand        string `json:"brand"`
	Manufacturer string `json:"manufacturer"`
	Uri          string `json:"uri"`
	//HdUri        string `json:"hd_uri"`
	//SdUri        string `json:"sd_uri"`
	UserName string `json:"username"`
	Password string `json:"password"`
	//UpdatedAt          string `json:"updated_at"`
	//LatestOnlineTime   string `json:"time"`
	UploadVideo string `json:"upload_video_enabled"`
	NvrSn       string `json:"nvr_sn"`
	Channel     string `json:"channel"`
	NC          string `json:"nc"`
	//DetectParams       model.DetectParams `json:"detect_params"`
	//detectCfg          *DetectConfig
}

type CameraSettingsStruct struct {
	CamID             string `json:"camera_id"`
	CamSN             string `json:"camera_serial_no"`
	Events            string `json:"cloud_event_types"`
	StreamSettings    string `json:"stream_settings"`
	VideoCapabilities string `json:"video_capabilities"`
	AudioSettings     string `json:"audio_settings"`
	OSDSettings       string `json:"osd_settings"`
}

type OnvifDeviceStruct struct {
	IP              string `json:"ip"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	Endpoints       string `json:"endpoints"`
	Manufacturer    string `json:"manufacturer"`
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmware version"`
	SerialNumber    string `json:"serial number"`
	HardwareId      string `json:"hardware id"`
	MACAddress      string `json:"mac address"`
}

func (d *DumpAPI) camera() []CameraStruct {
	cameras := d.Box.GetNVRManager().GetAICameras()
	cams := []CameraStruct{}
	sort.SliceStable(cameras, func(i, j int) bool {
		if cameras[i].GetChannel() < cameras[j].GetChannel() {
			return true
		}
		return false
	})
	for _, cam := range cameras {
		cams = append(cams, CameraStruct{
			ID: fmt.Sprintf("%d", cam.GetID()),
			IP: cam.GetIP(),
			SN: cam.GetSN(),
			//Version:      fmt.Sprintf("%d", cam.GetVersion()),
			Name:         cam.GetName(),
			Online:       fmt.Sprintf("%+v", cam.GetOnline()),
			Brand:        string(cam.GetBrand()),
			Manufacturer: cam.GetManufacturer(),
			Uri:          cam.GetUri(),
			//HdUri:        cam.GetHdUri(),
			//SdUri:        cam.GetSdUri(),
			UserName: cam.GetUserName(),
			Password: cam.GetPassword(),
			//UpdatedAt:          cam.GetUpdatedAt().String(),
			//LatestOnlineTime:   cam.GetLatestOnlineTime().String(),
			UploadVideo: fmt.Sprintf("%+v", cam.GetUploadVideoEnabled()),
			NvrSn:       cam.GetNvrSN(),
			Channel:     fmt.Sprintf("%d", cam.GetChannel()),
			NC:          fmt.Sprintf("%+v", cam.GetNVRClient()),
		})
	}
	return cams
}

func (d *DumpAPI) Cameras(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, d.camera())
}

func (d *DumpAPI) TCameras(ctx *gin.Context) {
	cams := d.camera()
	headers := []string{}
	headerInit := false
	data := [][]string{}
	for _, c := range cams {
		singleData := []string{}
		cReflected := reflect.TypeOf(c)
		for i := 0; i < cReflected.NumField(); i++ {
			field := cReflected.Field(i)
			if !headerInit {
				headers = append(headers, field.Name)
			}
			singleData = append(singleData, reflect.ValueOf(c).FieldByName(field.Name).String())
		}
		headerInit = true
		data = append(data, singleData)
	}
	buf := new(bytes.Buffer)
	table := tablewriter.NewWriter(buf)
	table.SetHeader(headers)
	table.AppendBulk(data)
	table.Render()
	ctx.String(http.StatusOK, buf.String())
}

func (d *DumpAPI) CameraSettingss(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, cloud.AllCameraSettings())
}

func (d *DumpAPI) flatCameraSettings(settings []*cloud.CameraSettings) []CameraSettingsStruct {
	ret := []CameraSettingsStruct{}
	for index, _ := range settings {
		setting := settings[index]
		ret = append(ret, CameraSettingsStruct{
			CamID:             fmt.Sprintf("%d", setting.CamID),
			CamSN:             setting.CamSN,
			Events:            fmt.Sprintf("%+v", setting.CloudEventTypes),
			StreamSettings:    fmt.Sprintf("%+v", setting.StreamSettings),
			VideoCapabilities: fmt.Sprintf("%+v", setting.VideoCapabilities),
			AudioSettings:     fmt.Sprintf("%+v", setting.AudioSettings),
			OSDSettings:       fmt.Sprintf("%+v", setting.OSDSettings),
		})
	}
	return ret
}

func (d *DumpAPI) TCameraSettingss(ctx *gin.Context) {
	settings := d.flatCameraSettings(cloud.AllCameraSettings())
	headers := []string{}
	headerInit := false
	data := [][]string{}
	for _, c := range settings {
		singleData := []string{}
		cReflected := reflect.TypeOf(c)
		for i := 0; i < cReflected.NumField(); i++ {
			field := cReflected.Field(i)
			if !headerInit {
				headers = append(headers, field.Name)
			}
			singleData = append(singleData, reflect.ValueOf(c).FieldByName(field.Name).String())
		}
		headerInit = true
		data = append(data, singleData)
	}
	buf := new(bytes.Buffer)
	table := tablewriter.NewWriter(buf)
	table.SetHeader(headers)
	table.AppendBulk(data)
	table.Render()
	ctx.String(http.StatusOK, buf.String())
}

func (d *DumpAPI) flatOnvifDevice(devices []onvif.Device) []OnvifDeviceStruct {
	ret := []OnvifDeviceStruct{}
	for index, _ := range devices {
		d := devices[index]
		ret = append(ret, OnvifDeviceStruct{
			IP:              d.Params.Xaddr,
			Username:        d.Params.Username,
			Password:        d.Params.Password,
			Endpoints:       fmt.Sprintf("%+v", d.Endpoints),
			Manufacturer:    d.Info.Manufacturer,
			Model:           d.Info.Model,
			FirmwareVersion: d.Info.FirmwareVersion,
			SerialNumber:    d.Info.SerialNumber,
			HardwareId:      d.Info.HardwareId,
			MACAddress:      d.Info.MACAddress,
		})
	}
	return ret
}

func (d *DumpAPI) Nvr(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, d.flatOnvifDevice(d.Box.GetSearcher().GetDevices()))
}

func (d *DumpAPI) TNvr(ctx *gin.Context) {
	devices := d.flatOnvifDevice(d.Box.GetSearcher().GetDevices())
	headers := []string{}
	headerInit := false
	data := [][]string{}
	for _, c := range devices {
		singleData := []string{}
		cReflected := reflect.TypeOf(c)
		for i := 0; i < cReflected.NumField(); i++ {
			field := cReflected.Field(i)
			if !headerInit {
				headers = append(headers, field.Name)
			}
			singleData = append(singleData, reflect.ValueOf(c).FieldByName(field.Name).String())
		}
		headerInit = true
		data = append(data, singleData)
	}
	buf := new(bytes.Buffer)
	table := tablewriter.NewWriter(buf)
	table.SetHeader(headers)
	table.AppendBulk(data)
	table.Render()
	ctx.String(http.StatusOK, buf.String())
}

func (d *DumpAPI) NvrManaged(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, d.Box.GetNVRManager().GetAllNvrManaged())
}

func (d *DumpAPI) TNvrManaged(ctx *gin.Context) {
	headers := []string{"sn", "ip", "port", "username", "password", "online", "brand", "state", "deviceLockUntil", "serviceLockUntil", "nc"}
	data := [][]string{}
	ret := d.Box.GetNVRManager().GetAllNvrManaged()
	for _, nvr := range ret {
		data = append(data, []string{
			nvr["sn"],
			nvr["ip"],
			nvr["port"],
			nvr["username"],
			nvr["password"],
			nvr["online"],
			nvr["brand"],
			nvr["state"],
			nvr["deviceLockUntil"],
			nvr["serviceLockUntil"],
			nvr["nc"],
		})
	}

	buf := new(bytes.Buffer)
	table := tablewriter.NewWriter(buf)
	table.SetHeader(headers)
	table.AppendBulk(data)
	table.Render()
	ctx.String(http.StatusOK, buf.String())
}

func (d *DumpAPI) ArchiveSetting(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, box.GetArchiveTaskRunner().DumpTasks())
}

func (d *DumpAPI) TArchiveSetting(ctx *gin.Context) {
	headers := []string{"key", "id", "camId", "status", "resolution", "videoExpire", "isLive", "createdAt", "createdTime", "type", "streamStart", "streamEnd", "runningStatus", "streamUrl"}
	data := [][]string{}
	ret := box.GetArchiveTaskRunner().DumpTasks()
	for _, task := range ret {
		data = append(data, []string{
			task["key"],
			task["id"],
			task["camId"],
			task["status"],
			task["resolution"],
			task["videoExpire"],
			task["isLive"],
			task["createdAt"],
			task["createdTime"],
			task["type"],
			task["streamStart"],
			task["streamEnd"],
			task["runningStatus"],
			task["streamUrl"],
		})
	}

	buf := new(bytes.Buffer)
	table := tablewriter.NewWriter(buf)
	table.SetHeader(headers)
	table.AppendBulk(data)
	table.Render()
	ctx.String(http.StatusOK, buf.String())
}

func (d *DumpAPI) foundCloudNvr() []map[string]string {
	nvrs := d.Box.ListCloudNvrs()
	ret := []map[string]string{}
	for idx := range nvrs {
		n := nvrs[idx]
		nvrDump := make(map[string]string)
		nvrDump["id"] = fmt.Sprintf("%d", n.ID)
		nvrDump["name"] = n.Name
		nvrDump["sn"] = n.SN
		nvrDump["host"] = n.Host
		nvrDump["mac"] = n.MacAddress
		nvrDump["manufacturer"] = n.Manufacturer
		nvrDump["username"] = n.Username
		nvrDump["password"] = n.Password
		nvrDump["model"] = n.DeviceModel
		if n.Online != nil {
			nvrDump["online"] = fmt.Sprintf("%+v", *n.Online)
		} else {
			nvrDump["online"] = ""
		}
		nvrDump["activate"] = n.ActivateStatus
		nvrDump["state"] = n.State
		if n.Timezone != nil {
			nvrDump["timezone"] = fmt.Sprintf("%+v", *n.Timezone)
		} else {
			nvrDump["timezone"] = ""
		}
		ret = append(ret, nvrDump)
	}
	return ret
}

func (d *DumpAPI) CloudNvr(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, d.foundCloudNvr())
}

func (d *DumpAPI) TCloudNvr(ctx *gin.Context) {
	headers := []string{"id", "name", "sn", "host", "mac", "manufacturer", "username", "password", "model", "online", "activate", "state", "timezone"}
	data := [][]string{}
	ret := d.foundCloudNvr()
	for _, task := range ret {
		data = append(data, []string{
			task["id"],
			task["name"],
			task["sn"],
			task["host"],
			task["mac"],
			task["manufacturer"],
			task["username"],
			task["password"],
			task["model"],
			task["online"],
			task["activate"],
			task["state"],
			task["timezone"],
		})
	}

	buf := new(bytes.Buffer)
	table := tablewriter.NewWriter(buf)
	table.SetHeader(headers)
	table.AppendBulk(data)
	table.Render()
	ctx.String(http.StatusOK, buf.String())
}
