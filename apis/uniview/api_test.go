package uniview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/example/turing-common/log"
	"github.com/example/turing-common/model"

	"github.com/example/minibox/camera/base"
	"github.com/example/minibox/camera/uniview"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/configs"
	"github.com/example/minibox/mock"
)

func getMockUniviewApi(box *mock.MockBox, db *mock.MockDBClient) *UniviewAPI {
	return &UniviewAPI{
		Box:    box,
		DB:     db,
		Logger: log.Logger("uniview_api"),
	}

}

type mockErrReader struct{}

func (m mockErrReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("mock error")
}

func TestRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	db := mock.NewMockDBClient(ctrl)
	box := mock.NewMockBox(ctrl)
	injector := configs.GetInjector()
	injector.Map(box)
	injector.Map(db)
	t.Run("register success", func(t *testing.T) {
		t.Parallel()
		Register(configs.GetInjector(), gin.Default())
	})
}

func TestUniviewAPI_BaseURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)
	u := getMockUniviewApi(box, db)
	url := u.BaseURL()
	assert.Equal(t, "LAPI/V1.0", url)
}

func TestUniviewAPI_Middlewares(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)
	u := getMockUniviewApi(box, db)
	mds := u.Middlewares()
	assert.EqualValues(t, []gin.HandlerFunc{}, mds)
}

func TestUniviewAPI_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)
	u := getMockUniviewApi(box, db)
	router := gin.Default().RouterGroup
	group := router.Group(u.BaseURL())
	u.Register(group)
}

func TestUniviewAPI_ShowContent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)
	u := getMockUniviewApi(box, db)
	g := gin.New()

	g.POST("/content", u.ShowContent)
	w := httptest.NewRecorder()

	t.Run("GetRawData error", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/content", mockErrReader{})
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/content", bytes.NewReader([]byte(`{"content":"val"}`)))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})
}

func TestUniviewAPI_HandleEventNotification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)
	cg := mock.NewMockCamGroup(ctrl)
	cfg := mock.NewMockConfig(ctrl)
	cfg.EXPECT().GetDataStoreDir().Return("./data").Times(1)
	cfg.EXPECT().GetEventIntervalSecs().Return(int64(10)).AnyTimes()
	cfg.EXPECT().GetDisableCloud().Return(false).AnyTimes()
	cfg.EXPECT().GetVideoClipDuration().Return(int64(7)).Times(1)
	cfg.EXPECT().GetSecBeforeEvent().Return(int64(2)).AnyTimes()
	c := uniview.NewCamera(&cloud.Camera{
		ID:           10,
		SN:           "test_sn",
		NvrSN:        "test_sn",
		Uri:          "rtsp://192.168.11.128/unicast/c1/s0/live",
		DetectParams: model.DetectParams{DetectAt: "cloud"},
	}, cfg)
	c.SetManufacturer("test_manufacturer")
	c.SetStatus(true)

	cg.EXPECT().AllCameras().Return([]base.Camera{c, c}).AnyTimes()

	box.EXPECT().GetCamGroup().Return(cg).AnyTimes()
	box.EXPECT().GetConfig().Return(cfg).AnyTimes()

	u := getMockUniviewApi(box, db)

	g := gin.Default()
	router := g.RouterGroup
	group := router.Group(u.BaseURL())
	u.Register(group)

	w := httptest.NewRecorder()

	t.Run("GetRawData error", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Structure", mockErrReader{})
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("unexpected end of json input", func(t *testing.T) {
		r := bytes.NewReader([]byte{})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Structure", r)
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("no camera found,no nvr sn", func(t *testing.T) {
		buff, _ := json.Marshal(EventNotification{Reference: "ttt/122123"})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Structure", bytes.NewReader(buff))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		buff, _ := json.Marshal(EventNotification{})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Structure", bytes.NewReader(buff))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	c.SetManufacturer("Turing-U")
	t.Run("success", func(t *testing.T) { // Triggering Normal Event Pipeline
		buff, _ := json.Marshal(EventNotification{})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Structure", bytes.NewReader(buff))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("success", func(t *testing.T) { // Triggering Full Event Pipeline
		buff, _ := json.Marshal(EventNotification{
			Reference:        "",
			Timestamp:        123,
			Seq:              123,
			SrcID:            0,
			SrcName:          "EnterArea",
			NotificationType: 0,
			DeviceID:         "foober",
			RelatedID:        "goober",
			StructureInfo: StructureDataInfo{
				ObjInfo: ObjectInfo{
					FaceNum:                 0,
					FaceInfoList:            nil,
					PersonNum:               1,
					PersonInfoList:          nil,
					NonMotorVehicleNum:      0,
					NonMotorVehicleInfoList: nil,
					VehicleNum:              0,
					VehicleInfoList:         nil,
				},
				ImageNum:       0,
				ImageInfoList:  nil,
				FinishFaceNum:  0,
				FinishFaceList: nil,
			},
		})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Structure", bytes.NewReader(buff))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("filter within 10s event success", func(t *testing.T) {
		cfg.EXPECT().GetEventSavedHours().Times(1)
		buff, _ := json.Marshal(EventNotification{
			Reference: "192.168.11.176:58081/test_sn/Subscription/Subscribers/19",
			Timestamp: 1681803328,
			SrcID:     1,
			SrcName:   "EnterArea",
		})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Structure", bytes.NewReader(buff))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)

		buff2, _ := json.Marshal(EventNotification{
			Reference: "192.168.11.176:58081/test_sn/Subscription/Subscribers/19",
			Timestamp: 1681803330,
			SrcID:     1,
			SrcName:   "EnterArea",
		})
		req, _ = http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Structure", bytes.NewReader(buff2))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})
}

func TestUniviewAPI_UploadAlarm(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)
	cg := mock.NewMockCamGroup(ctrl)
	cfg := configs.NewEmptyConfig()
	c := uniview.NewCamera(&cloud.Camera{ID: 10, SN: "test_sn", DetectParams: model.DetectParams{DetectAt: "cloud"}}, &cfg)
	c.SetManufacturer("test_manufacturer")

	cg.EXPECT().AllCameras().Return([]base.Camera{c, c}).AnyTimes()

	box.EXPECT().GetCamGroup().Return(cg).AnyTimes()
	box.EXPECT().GetConfig().Return(&cfg).AnyTimes()

	u := getMockUniviewApi(box, db)

	g := gin.Default()
	router := g.RouterGroup
	group := router.Group(u.BaseURL())
	u.Register(group)

	w := httptest.NewRecorder()

	t.Run("GetRawData error", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Alarm", mockErrReader{})
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("unexpected end of json input", func(t *testing.T) {
		r := bytes.NewReader([]byte{})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Alarm", r)
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("no sn", func(t *testing.T) {
		buff, _ := json.Marshal(AlarmNotification{})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Alarm", bytes.NewReader(buff))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("no camera", func(t *testing.T) {
		buff, _ := json.Marshal(AlarmNotification{Reference: "tt/122", AlarmInfo: AlarmInfo{AlarmType: MotionAlarmOff}})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/Alarm", bytes.NewReader(buff))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})
}

func TestUniviewAPI_UploadMotionDetection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	box := mock.NewMockBox(ctrl)
	db := mock.NewMockDBClient(ctrl)
	cg := mock.NewMockCamGroup(ctrl)
	cfg := configs.NewEmptyConfig()
	c := uniview.NewCamera(&cloud.Camera{ID: 10, SN: "test_sn"}, &cfg)
	cg.EXPECT().AllCameras().Return([]base.Camera{c, c}).AnyTimes()

	box.EXPECT().GetCamGroup().Return(cg).AnyTimes()
	box.EXPECT().GetConfig().Return(&cfg).AnyTimes()

	u := getMockUniviewApi(box, db)

	g := gin.Default()
	router := g.RouterGroup
	group := router.Group(u.BaseURL())
	u.Register(group)

	w := httptest.NewRecorder()

	t.Run("GetRawData error", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/MotionDetection", mockErrReader{})
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("unexpected end of json input", func(t *testing.T) {
		r := bytes.NewReader([]byte{})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/MotionDetection", r)
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("no image", func(t *testing.T) {
		buff, _ := json.Marshal(MotionDetectionAlarm{})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/MotionDetection", bytes.NewReader(buff))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("no sn", func(t *testing.T) {
		buff, _ := json.Marshal(MotionDetectionAlarm{AlarmPicture: AlarmPicture{ImageList: []ImageInfo{{Data: data}}}})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/MotionDetection", bytes.NewReader(buff))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("no camera", func(t *testing.T) {
		buff, _ := json.Marshal(MotionDetectionAlarm{Reference: "ttt/1212", AlarmPicture: AlarmPicture{ImageList: []ImageInfo{{Data: data}}}})
		req, _ := http.NewRequest("POST", "/LAPI/V1.0/System/Event/Notification/MotionDetection", bytes.NewReader(buff))
		g.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})
}
