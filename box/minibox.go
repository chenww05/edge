package box

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/example/turing-common/env"
	"github.com/example/turing-common/log"
	"github.com/example/turing-common/websocket"

	"github.com/example/minibox/apis/structs"
	"github.com/example/minibox/camera"
	"github.com/example/minibox/camera/base"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/configs"
	"github.com/example/minibox/db"
	"github.com/example/minibox/discover"
	"github.com/example/minibox/discover/arp"
	"github.com/example/minibox/nvr"
	"github.com/example/minibox/printer"
	"github.com/example/minibox/utils"
)

var ErrInit = errors.New("init error")

const (
	retryMinDelay         int64  = 1
	retryMaxDelay         int64  = 30
	PurposeThermal        string = "thermal"
	TokenNameCameraSnap   string = "camera_snaps"
	TokenNameCameraEvent  string = "camera_events"
	TokenNameCameraVideo  string = "camera_videos"
	TokenNameCloudStorage string = "cloud_storage"
	TimeFormat            string = "2006-01-02 15:04:05"
	DateFormat            string = "2006-01-02"
	TimeOffsetFormat      string = "GMT-07:00:00"
	TimeFormatFromCamera  string = "2006-01-02T15:04:05"
)

var defaultAlgoMap map[string]string

func init() {
	defaultAlgoMap = map[string]string{
		// {"event_type_code_name": "event_type_code_name:event_type_id"}
		"car":                  "car:2",
		"temperature_abnormal": "temperature_abnormal:91",
		"temperature_normal":   "temperature_normal:92",
		"face_tracking":        "face_tracking:101",
		"license_plate":        "license_plate:104",
		"questionnaire_fail":   "questionnaire_fail:106",
		"human_count":          "human_count:107",
		"human_count_stat":     "human_count_stat:108",
		"human_count_exceed":   "human_count_exceed:109",
		"intrude":              "intrude:110",
		"fire":                 "fire:105",
		"zawu":                 "zawu:111",
		"car_blocking":         "car_blocking:113",
		"abnormal":             "abnormal:115",
		"vacant":               "vacant:114",
		"lying_down":           "lying_down:116",
		"motorcycle_intrude":   "motorcycle_intrude:117",
		"motorcycle_enter":     "motorcycle_enter:118",
		"motion_start":         "motion_start:119",
		"people_count":         "people_count:120",
	}
}

type Box interface {
	Start()
	Init() error
	UpdateCameras() error
	GetTokenByBox(string) (*cloud.Token, error)
	GetTokenByCamera(int, string) (*cloud.Token, error)
	UploadS3ByTokenName(int, string, int, int, string, string) (*utils.S3File, error)
	UploadCameraEvent(int, *utils.S3File, float64, float64, *structs.MetaScanData, time.Time) (string, error)
	UploadPplEvent(cameraID int, meta *structs.MetaScanData, startAt time.Time, endedAt time.Time, eventType string) (string, error)
	UploadAICameraEvent(cameraID int, file *utils.S3File, startAt, endAt, timestamp time.Time, eventType string, meta *structs.MetaScanData) (string, error)
	NotifyCloudEventVideoClipUploadFailed(id int) error
	UploadEventMedia(string, *cloud.Media) error
	UploadCameraSnapshot(*cloud.CamSnapShotReq) error
	GetCamera(cameraID int) (base.Camera, error)
	GetCameraBySN(sn string) (base.Camera, error)
	AddCamera(c base.Camera)
	GetConfig() configs.Config
	GetCamGroup() camera.CamGroup
	UpdateAPIClientConfig() error
	ConnectCloud() error
	WsClient() websocket.Client
	DisconnectCloud()
	GetPrintStrategy() printer.PrintStrategy
	CloudClient() cloud.Client
	SetUploadConfig(configs.UploadConfig) error
	SetNVRManager(nvrManager nvr.NVRManager)
	GetNVRManager() nvr.NVRManager
	UpdateNVR(sn string, user string, password string, brand utils.CameraBrand, ip string, port uint16)
	SetSearcher(searcher *discover.SearcherProcessor)
	GetSearcher() *discover.SearcherProcessor
	SetArpSearcher(searcher arp.SearcherProcessor)
	GetArpSearcher() arp.SearcherProcessor
	GetAllRecords(sn string, channel uint32, begin int64, end int64, data interface{}) error
	GetRecordsDaily(sn string, channel, year, month uint32, data interface{}) error
	ObjectDetect(imgData string) (error, []utils.DetectObjects)
	GetBoxId() string
	GetSrsIp() (string, error)
	GetSdpRemote(srsIp string, srsPort int64, sdpLocal string, cameraId int, streamId string) (string, error)
	SetCloudNvrs(nvrs []*cloud.NVR)
	GetCloudNvr(sn string) (*cloud.NVR, error)
	ListCloudNvrs() []*cloud.NVR
}

func New(cfg configs.Config, boxInfo configs.BoxInfo, db db.Client) Box {
	box := newBox(cfg, &boxInfo, db)
	return box
}

func (b *baseBox) GetSrsIp() (string, error) {
	net_list, e := utils.GetNetworkInterfaces()
	if e != nil {
		return "", e
	}
	srsIp := ""
	for _, net := range net_list {
		if strings.HasPrefix(net.Name, "en") {
			srsIp = net.IPv4List[0].IP
			break
		}
		if strings.HasPrefix(net.Name, "wl") {
			srsIp = net.IPv4List[0].IP
		}
	}
	if srsIp == "" {
		errEmpty := errors.New("ip empty")
		return "", errEmpty
	}
	b.logger.Info().Msgf("get local ip %s", srsIp)
	return srsIp, nil
}

func (b *baseBox) GetSdpRemote(srsIp string, srsPort int64, sdpLocal string, cameraId int, streamId string) (string, error) {
	urlPath := defaultPlayPath
	if strings.Contains(sdpLocal, sdpH265KeyWord) {
		urlPath = defaultDataPath
	}
	srsUrl := fmt.Sprintf("http://%s:%d/%s/?eip=%s", srsIp, srsPort, urlPath, srsIp)

	// send to local srs here
	apiReq := srsApiReq{
		ClientIp:  "",
		Sdp:       sdpLocal,
		StreamUrl: fmt.Sprintf("webrtc://%s/live/%d/%s", srsIp, cameraId, streamId),
	}
	msgBytes, err := json.Marshal(apiReq)
	if err != nil {
		b.logger.Error().Msgf("json Marshal, error = %s", err.Error())
		return "", err
	}

	bodyData, err := b.srs.GetSdpRemote(srsUrl, msgBytes)
	if err != nil {
		b.logger.Error().Msgf("failed to GetSdpRemote with error = %v", err)
		return "", err
	}

	var info = srsApiRsp{}
	err = json.Unmarshal(bodyData, &info)
	if err != nil {
		b.logger.Error().Msgf("json Unmarshal error = %s", err.Error())
		return "", err
	}

	return info.Sdp, nil
}

func (b *baseBox) GetAllRecords(sn string, channel uint32, begin int64, end int64, data interface{}) error {
	return b.nvrManager.GetAllRecords(sn, channel, begin, end, data)
}

func (b *baseBox) GetRecordsDaily(sn string, channel, year, month uint32, data interface{}) error {
	return b.nvrManager.GetRecordsDaily(sn, channel, year, month, data)
}

func (b *baseBox) cleanStatics() {
	// box stop,but ffmpeg not stop,ffmpeg may clip video file,this case video not delete
	storeDir := b.config.GetDataStoreDir()
	if storeDir != "" {
		if err := utils.DeleteDirFileWithSuffix(storeDir, "mp4", "jpg"); err != nil {
			b.logger.Err(err).Msg("delete file error")
		}
		b.logger.Info().Msgf("clear %s success ", storeDir)
	}
}

func (b *baseBox) Start() {
	b.cleanStatics()
	delay := retryMinDelay
	for {
		err := b.Init()
		if err == nil {
			break
		}

		if e, ok := err.(cloud.Err); ok {
			/**
			  EC_AGENT = 101
			  EC_METHOD = 102
			  EC_INPUT = 103
			  EC_NOT_IN = 104
			  EC_FORBID = 105
			  EC_LOGIC = 106
			  EC_PERM = 107
			  EC_RESTRICT = 108
			*/
			if e.Code >= 100 && e.Code <= 110 {
				b.logger.Error().Msgf("Stopped init due to: %s", err)
				break
			}
		}

		b.logger.Error().Msgf("Failed to init: %s", err)
		time.Sleep(time.Duration(delay) * time.Second)
		delay *= 2
		if delay > retryMaxDelay {
			delay = retryMaxDelay
		}
	}
	b.nvrManager.Start()
}

type baseBox struct {
	env.Config      `json:"-"`
	config          configs.Config
	boxInfo         *configs.BoxInfo
	logger          zerolog.Logger
	apiClient       cloud.Client
	wsClient        websocket.Client
	s3httpClient    *http.Client
	algoMap         *sync.Map
	camGroup        camera.CamGroup
	cloudNvrGroup   *sync.Map
	configMux       *sync.Mutex
	disconnectCloud context.CancelFunc
	db              db.Client
	mux             *sync.Mutex
	nvrManager      nvr.NVRManager
	ctx             context.Context
	cancel          context.CancelFunc
	searcher        *discover.SearcherProcessor
	arpSearcher     arp.SearcherProcessor
	tokenCache      *sync.Map
	srs             Srs
}

func (b *baseBox) UpdateNVR(sn string, user string, password string, brand utils.CameraBrand, ip string, port uint16) {
	b.nvrManager.StoreNVRToDB(sn, user, password, ip, brand, port)
}

func newBox(cfg configs.Config, boxInfo *configs.BoxInfo, db db.Client) *baseBox {
	cameraTimeoutSecs := cfg.GetCameraTimeoutSecs()
	box := &baseBox{
		config:        cfg,
		boxInfo:       boxInfo,
		logger:        log.Logger("box"),
		s3httpClient:  &http.Client{},
		algoMap:       &sync.Map{},
		configMux:     &sync.Mutex{},
		camGroup:      camera.NewCamGroup(cameraTimeoutSecs),
		cloudNvrGroup: &sync.Map{},
		db:            db,
		mux:           &sync.Mutex{},
		tokenCache:    &sync.Map{},
		srs:           &baseSrs{},
	}
	for k, algo := range defaultAlgoMap {
		box.algoMap.Store(k, algo)
	}
	return box
}

func (b *baseBox) ConnectCloud() error {
	err := b.connectAPIServer()
	if err != nil {
		return err
	}

	if err := b.connectWebsocketServer(); err != nil {
		return err
	}

	b.logger.Info().Msg("connected to cloud services")

	return nil
}

func (b *baseBox) DisconnectCloud() {
	if b.disconnectCloud != nil {
		b.logger.Info().Msg("disconnecting from cloud")
		b.disconnectCloud()
	}
}

func (b *baseBox) connectAPIServer() error {
	config, err := b.newClientConfig()
	if err != nil {
		return err
	}

	if b.apiClient == nil {
		b.apiClient = cloud.NewClient(*config)
	}
	return nil
}

func (b *baseBox) getLastCookies() []*http.Cookie {
	if err := b.apiClient.Handshake(); err != nil {
		return nil
	}
	return b.apiClient.Cookies()
}

func (b *baseBox) connectWebsocketServer() error {
	cfg := b.GetConfig()
	agent := fmt.Sprintf("Box/%d/%s", cfg.GetLevel(), cfg.GetSoftwareVersion())

	wsClient, err := websocket.NewClientWithOptions(context.TODO(), websocket.ConstructorArgs{
		WebsocketURL: cfg.GetWebsocketServerUrl(),
		GetCookies:   b.getLastCookies,
		Agent:        agent,
		Handler:      NewHandler(b),
	}, websocket.PingPeriod(cfg.GetWebsocketPingPeriod()),
		websocket.ReconnectSleep(cfg.GetWebsocketReconnectSleepPeriod()))
	if err != nil {
		return err
	}

	b.wsClient = wsClient
	return nil
}

func (b *baseBox) Init() error {
	if b.ctx != nil && b.cancel != nil {
		b.cancel()
	}
	b.ctx, b.cancel = context.WithCancel(context.Background())
	if !b.GetConfig().GetDisableCloud() {
		err := b.ConnectCloud()
		if err != nil {
			return err
		}

		err = b.UpdateCameras()
		if err != nil {
			b.DisconnectCloud()
			return err
		}

		go b.uploadBoxTimezone(b.ctx)
		go b.uploadCameraState(b.ctx)
		go b.retryUploadEvents(b.ctx)
		go b.syncCameraSettings(b.ctx)
		go b.syncIotDevices(b.ctx)
		b.disconnectCloud = b.cancel
	} else {
		b.logger.Info().Msg("config disabled cloud, skipping init")
	}
	return nil
}

func (b *baseBox) UpdateCameras() error {
	// Fetch Nvrs from cloud when cloud device changed
	if cloudNvrs, nvrErr := b.CloudClient().GetNvrs(); nvrErr != nil {
		b.logger.Error().Err(nvrErr).Msgf("get nvrs from cloud failed")
	} else {
		b.logger.Debug().Msgf("get nvrs from cloud success, nvrs: %+v", cloudNvrs)
		b.SetCloudNvrs(cloudNvrs)
	}

	err := b.updateCameras()
	if err != nil {
		return err
	}

	b.logger.Debug().Msg("start to sync camera settings.")

	go func() {
		b.updateRemoteCameraSettings()
		b.updateLocalCameraSettings()
	}()
	return nil
}

func (b *baseBox) updateCameras() error {
	b.mux.Lock()
	defer b.mux.Unlock()
	logger := b.logger.With().Str("init", "cameras").Logger()
	if b.apiClient == nil {
		return ErrNoAPIClient
	}

	cloudCams := make(map[string]struct{})
	localCams, err := b.apiClient.GetCameras()
	if err != nil {
		logger.Error().Err(err).Msg("init failed to get camera error")
		return err
	}

	for _, c := range localCams {
		logger := logger.With().Str("camera_sn", c.SN).Logger()
		cloudCams[c.SN] = struct{}{}

		var cb utils.CameraBrand
		if len(c.Brand) == 0 {
			logger.Info().Msg("camera has no brand information, defaulting to thermal 1")
			cb = utils.Thermal1
		} else {
			cb = utils.CameraBrand(c.Brand)
		}
		if c.ID <= 0 || len(c.SN) <= 0 {
			logger.Error().Int("id", c.ID).Msgf("invalid camera: %+v, either SN or ID is empty", c)
			continue
		}

		var cam base.Camera
		if old, _ := b.GetCameraBySN(c.SN); old != nil {
			old.UpdateCamera(c.SN, c.ID)
			logger.Info().Int("camera_id", c.ID).Msg("updated existing camera with cloud ID")
			cam = old
		} else {
			logger.Info().Str("sn", c.SN).Msg("no camera found,then new camera")
			cam = camera.NewCamera(c, cb, b.config)
			b.AddCamera(cam)
		}
		aiCam, ok := cam.(base.AICamera)
		if ok {
			newAiCam := camera.NewAICamera(c, cb, b.config)
			b.logger.Debug().Msg("new camera to merge")
			if err := aiCam.MergeCamera(newAiCam); err != nil {
				b.logger.Err(err).Msg("merge camera error")
				continue
			}
			//	set nvrSN
			aiCam.SetNvrSN(c.NvrSN)
		}
	}

	// if api client is not initialized during heartbeat, camera will be added to cloud here
	cameras := b.GetCamGroup().AllCameras()
	for _, c := range cameras {
		// Only for Thermal
		if c.GetID() < 1 {
			if cloudCam, err := b.apiClient.AddCamera(c.GetSN(), string(c.GetBrand()), c.GetIP()); err != nil {
				b.logger.Error().Err(err).Msg("UpdateCameras: error adding camera from heartbeat")
			} else {
				logger.Info().Int("camera_id", cloudCam.ID).Str("camera_sn", cloudCam.SN).Msg("added camera to cloud")
				c.UpdateCamera(cloudCam.SN, cloudCam.ID)
				logger.Info().Int("camera_id", cloudCam.ID).Str("camera_sn", cloudCam.SN).Msg("updated existing camera with cloud ID")
			}
		}
		// Only for Vision
		_, isAiCam := c.(base.AICamera)
		if _, ok := cloudCams[c.GetSN()]; !ok && isAiCam {
			logger.Info().Int("camera_id", c.GetID()).Str("camera_sn", c.GetSN()).Msg("local camera " +
				"does not exist in cloud, deleting camera locally")
			b.camGroup.DelCamera(c.GetSN())
		}
	}
	if b.nvrManager == nil {
		return nil
	}

	logger.Info().Msg("try to UpdateNvr when UpdateCameras")
	// Uri and nvr client already updated after nm.RefreshNVR
	b.nvrManager.UpdateNvrSync()
	b.nvrManager.HeartBeat()
	return nil
}

func (b *baseBox) GetConfig() configs.Config {
	b.configMux.Lock()
	defer b.configMux.Unlock()
	return b.config
}

func (b *baseBox) UpdateAPIClientConfig() error {
	if b.apiClient == nil {
		return ErrNoAPIClient
	}

	clientConfig, err := b.newClientConfig()
	if err != nil {
		return err
	}

	b.apiClient.UpdateClientConfig(*clientConfig)
	return nil
}

func (b *baseBox) newClientConfig() (*cloud.ClientConfig, error) {
	cfg := b.GetConfig()

	u, err := url.Parse(cfg.GetAPIServer())
	if err != nil {
		return nil, err
	}

	upload := cfg.GetUploadConfig()
	return &cloud.ClientConfig{
		Level:                    cfg.GetLevel(),
		Version:                  cfg.GetSoftwareVersion(),
		CloudServerURL:           u,
		DeviceID:                 b.boxInfo.ID,
		Hash:                     b.boxInfo.Hash,
		PrivateKey:               b.boxInfo.PrivateKey,
		DeviceType:               b.boxInfo.Type,
		IP:                       "127.0.0.1",
		DisableUploadTemperature: upload.DisableUploadTemperature,
		DisableUploadPic:         upload.DisableUploadPic,
		TimeZone:                 cfg.GetTimeZone(),
	}, nil
}

func (b *baseBox) GetPrintStrategy() printer.PrintStrategy {
	c := b.GetConfig()
	tz, _ := utils.LoadUTCLocation(c.GetTimeZone())
	return printer.NewPrintStrategy(b.logger, c.GetPrinterConfig(), *tz, c.GetTemperatureUnit(), c.GetPrinterConfig().IniConfig.Server)
}

func (b *baseBox) SetUploadConfig(cfg configs.UploadConfig) error {
	config := b.GetConfig()
	upload := config.GetUploadConfig()

	wasDisabled := upload.DisableCloud
	err := config.SetUploadConfig(&cfg)
	if err != nil {
		return err
	}

	if wasDisabled != cfg.DisableCloud {
		if !cfg.DisableCloud {
			err := b.Init()
			if err != nil {
				b.logger.Error().Msgf("ConnectCloud error: %s", err)
				_ = config.SetUploadConfig(upload)
				return ErrInit
			}
		} else {
			b.DisconnectCloud()
		}
	} else {
		if !cfg.DisableCloud {
			err = b.UpdateAPIClientConfig()
			if err != nil {
				b.logger.Error().Msgf("UpdateAPIClientConfig: %s", err)
				_ = config.SetUploadConfig(upload)
				return err
			}
		}
	}

	return nil
}

func (b *baseBox) SetNVRManager(nvrManager nvr.NVRManager) {
	b.nvrManager = nvrManager
}

func (b *baseBox) GetNVRManager() nvr.NVRManager {
	return b.nvrManager
}

func (b *baseBox) RefreshDevices() {
	b.nvrManager.RefreshDevice()
}

func (b *baseBox) SetSearcher(searcher *discover.SearcherProcessor) {
	b.searcher = searcher
}

func (b *baseBox) GetSearcher() *discover.SearcherProcessor {
	return b.searcher
}

func (b *baseBox) SetArpSearcher(searcher arp.SearcherProcessor) {
	b.arpSearcher = searcher
}

func (b *baseBox) GetArpSearcher() arp.SearcherProcessor {
	return b.arpSearcher
}

func (b *baseBox) GetBoxId() string {
	if b.boxInfo == nil {
		return ""
	}
	return b.boxInfo.ID
}

func (b *baseBox) SetCloudNvrs(nvrs []*cloud.NVR) {
	b.mux.Lock()
	defer b.mux.Unlock()
	b.cloudNvrGroup = &sync.Map{}
	for idx := range nvrs {
		n := nvrs[idx]
		b.cloudNvrGroup.Store(n.SN, *n)
	}
}

func (b *baseBox) GetCloudNvr(sn string) (*cloud.NVR, error) {
	if val, loadOk := b.cloudNvrGroup.Load(sn); !loadOk {
		return nil, fmt.Errorf("nvr not found from cloud nvr group, sn %s", sn)
	} else {
		if n, ok := val.(cloud.NVR); !ok {
			return nil, fmt.Errorf("cloud nvr data broken, sn %s", sn)
		} else {
			return &n, nil
		}
	}
}

func (b *baseBox) ListCloudNvrs() []*cloud.NVR {
	ret := []*cloud.NVR{}
	b.cloudNvrGroup.Range(func(key, value any) bool {
		if n, ok := value.(cloud.NVR); ok {
			ret = append(ret, &n)
		}
		return true
	})
	return ret
}

type Srs interface {
	GetSdpRemote(srsUrl string, msgBytes []byte) ([]byte, error)
}

type baseSrs struct {
}

func (s *baseSrs) GetSdpRemote(srsUrl string, msgBytes []byte) ([]byte, error) {
	res, err := http.Post(srsUrl, "application/json", bytes.NewBuffer(msgBytes))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	bodyData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return bodyData, nil
}
