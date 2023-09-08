package thermal_1_test

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/codegangsta/inject"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turingvideo/minibox/apis/common"
	t1 "github.com/turingvideo/minibox/apis/thermal_1"
	camera2 "github.com/turingvideo/minibox/camera"
	"github.com/turingvideo/minibox/camera/thermal_1"
	"github.com/turingvideo/minibox/cloud"
	"github.com/turingvideo/minibox/mock"
)

func TestRFIDForTimeKeeping(t *testing.T) {
	t.Run("sends to all scanners", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		gin.SetMode(gin.TestMode)
		engine := gin.Default()

		box := mock.NewMockBox(ctrl)
		cfg := mock.NewMockConfig(ctrl)
		cc := mock.NewMockClient(ctrl)
		box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
		box.EXPECT().CloudClient().Return(cc)
		cfg.EXPECT().GetTimeKeepingEnable().Return(true)

		injector := inject.New()
		injector.Map(common.ThermalBaseAPI{
			Box: box,
		})

		t1.Register(injector, engine)

		data := map[string]interface{}{
			"cmd":  t1.CmdCardReader,
			"rfid": "test_rfid",
		}

		testRfid := cloud.RfIdInfo{
			RfId: "test_rfid",
		}
		testEmployee := cloud.EmployeeInfo{
			ID:          "1",
			FirstName:   "first_name",
			MiddleName:  "middle_name",
			LastName:    "last_name",
			FullName:    "full_name",
			Sex:         "male",
			NO:          "No.1",
			RfId:        "test_rfid",
			ClockStatus: "new",
		}
		cc.EXPECT().RecognizeRfId(gomock.Eq(&testRfid)).Return(&testEmployee, nil)

		count := 0

		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count++
			var payload map[string]interface{}
			defer r.Body.Close()
			if assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				assert.Equal(t, map[string]interface{}{
					"clock_req_status": float64(1),
					"cmd":              t1.CmdCardReader,
					"company":          "",
					"head_shot":        "",
					"id":               "No.1",
					"name":             "full_name",
					"pass":             true,
					"rfid":             "test_rfid",
					"version":          t1.CmdVersion,
				}, payload)
			}

			w.WriteHeader(http.StatusOK)
			w.Write(nil)
		}))

		var err error
		s.Listener, err = net.Listen("tcp", ":8000")
		require.NoError(t, err)

		s.Start()
		defer s.Close()

		cg := camera2.NewCamGroup(0)
		cg.AddCamera(thermal_1.NewLiveCamera("test_sn", "localhost", ""))
		cg.AddCamera(thermal_1.NewLiveCamera("test_sn2", "localhost", ""))

		box.EXPECT().GetCamGroup().Return(cg)

		sendThermal1Request(t, data, engine)
		assert.Equal(t, 2, count)
	})
}

func TestHandleTimeClock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gin.SetMode(gin.TestMode)
	engine := gin.Default()

	box := mock.NewMockBox(ctrl)

	injector := inject.New()
	injector.Map(common.ThermalBaseAPI{
		Box: box,
	})

	t1.Register(injector, engine)

	cg := camera2.NewCamGroup(0)
	cg.AddCamera(thermal_1.NewLiveCamera("test_sn", "localhost", ""))

	cam, _ := cg.GetCameraBySN("test_sn")
	box.EXPECT().GetCameraBySN(gomock.Eq("test_sn")).Return(cam, nil).MinTimes(1)

	t.Run("time clock - clock in success", func(t *testing.T) {
		data := map[string]interface{}{
			"cmd":               t1.CmdTimeClock,
			"version":           t1.CmdVersion,
			"device_sn":         "test_sn",
			"timestamp":         1607414000,
			"rfid":              "test_rfid",
			"clock_resp_status": 1,
		}

		cc := mock.NewMockClient(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(1)

		testClockRecord := cloud.ClockRecord{
			TimeStamp:  1607414000,
			EmployeeID: "",
			RfId:       "test_rfid",
			Action:     "clock_in",
		}

		box.EXPECT().CloudClient().Return(cc)
		cc.EXPECT().AddClockRecord(gomock.Eq(&testClockRecord)).Return(nil)

		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			defer r.Body.Close()
			if assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				assert.Equal(t, map[string]interface{}{
					"version":      t1.CmdVersion,
					"cmd":          t1.CmdTimeClockResp,
					"clock_status": float64(1),
					"code":         float64(0),
				}, payload)
			}

			w.WriteHeader(http.StatusOK)
			w.Write(nil)
		}))

		var err error
		s.Listener, err = net.Listen("tcp", ":8000")
		require.NoError(t, err)

		s.Start()
		defer s.Close()

		sendThermal1Request(t, data, engine)
	})

	t.Run("time clock - clock in failed", func(t *testing.T) {
		data := map[string]interface{}{
			"cmd":               t1.CmdTimeClock,
			"version":           t1.CmdVersion,
			"device_sn":         "test_sn",
			"timestamp":         1607414000,
			"rfid":              "test_rfid",
			"clock_resp_status": 1,
		}

		// c := mock.NewMockThermal1Camera(ctrl)
		cc := mock.NewMockClient(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(1)

		testClockRecord := cloud.ClockRecord{
			TimeStamp:  1607414000,
			EmployeeID: "",
			RfId:       "test_rfid",
			Action:     "clock_in",
		}

		box.EXPECT().CloudClient().Return(cc)
		cc.EXPECT().AddClockRecord(gomock.Eq(&testClockRecord)).Return(errors.New("AddClockRecord ERROR"))
		// c.EXPECT().GetIP().Return(testIP).MinTimes(1)

		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			defer r.Body.Close()
			if assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				assert.Equal(t, map[string]interface{}{
					"version":      t1.CmdVersion,
					"cmd":          t1.CmdTimeClockResp,
					"clock_status": float64(1),
					"code":         float64(-1),
				}, payload)
			}

			w.WriteHeader(http.StatusOK)
			w.Write(nil)
		}))

		var err error
		s.Listener, err = net.Listen("tcp", ":8000")
		require.NoError(t, err)

		s.Start()
		defer s.Close()

		cg := camera2.NewCamGroup(0)
		cg.AddCamera(thermal_1.NewLiveCamera("test_sn", "localhost", ""))

		sendThermal1Request(t, data, engine)
	})

	t.Run("time clock - clock out success", func(t *testing.T) {
		data := map[string]interface{}{
			"cmd":               t1.CmdTimeClock,
			"version":           t1.CmdVersion,
			"device_sn":         "test_sn",
			"timestamp":         1607414000,
			"rfid":              "test_rfid",
			"clock_resp_status": 2,
		}

		// c := mock.NewMockThermal1Camera(ctrl)
		cc := mock.NewMockClient(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(1)

		testClockRecord := cloud.ClockRecord{
			TimeStamp:  1607414000,
			EmployeeID: "",
			RfId:       "test_rfid",
			Action:     "clock_out",
		}

		box.EXPECT().CloudClient().Return(cc)
		cc.EXPECT().AddClockRecord(gomock.Eq(&testClockRecord)).Return(nil)
		// c.EXPECT().GetIP().Return(testIP).MinTimes(1)

		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			defer r.Body.Close()
			if assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				assert.Equal(t, map[string]interface{}{
					"version":      t1.CmdVersion,
					"cmd":          t1.CmdTimeClockResp,
					"clock_status": float64(2),
					"code":         float64(0),
				}, payload)
			}

			w.WriteHeader(http.StatusOK)
			w.Write(nil)
		}))

		var err error
		s.Listener, err = net.Listen("tcp", ":8000")
		require.NoError(t, err)

		s.Start()
		defer s.Close()

		cg := camera2.NewCamGroup(0)
		cg.AddCamera(thermal_1.NewLiveCamera("test_sn", "localhost", ""))

		sendThermal1Request(t, data, engine)
	})

	t.Run("time clock - clock out failed", func(t *testing.T) {
		data := map[string]interface{}{
			"cmd":               t1.CmdTimeClock,
			"version":           t1.CmdVersion,
			"device_sn":         "test_sn",
			"timestamp":         1607414000,
			"rfid":              "test_rfid",
			"clock_resp_status": 2,
		}

		// c := mock.NewMockThermal1Camera(ctrl)
		cc := mock.NewMockClient(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(1)

		testClockRecord := cloud.ClockRecord{
			TimeStamp:  1607414000,
			EmployeeID: "",
			RfId:       "test_rfid",
			Action:     "clock_out",
		}

		box.EXPECT().CloudClient().Return(cc)
		cc.EXPECT().AddClockRecord(gomock.Eq(&testClockRecord)).Return(errors.New("AddClockRecord ERROR"))
		// c.EXPECT().GetIP().Return(testIP).MinTimes(1)

		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			defer r.Body.Close()
			if assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				assert.Equal(t, map[string]interface{}{
					"version":      t1.CmdVersion,
					"cmd":          t1.CmdTimeClockResp,
					"clock_status": float64(2),
					"code":         float64(-1),
				}, payload)
			}

			w.WriteHeader(http.StatusOK)
			w.Write(nil)
		}))

		var err error
		s.Listener, err = net.Listen("tcp", ":8000")
		require.NoError(t, err)

		s.Start()
		defer s.Close()

		cg := camera2.NewCamGroup(0)
		cg.AddCamera(thermal_1.NewLiveCamera("test_sn", "localhost", ""))

		sendThermal1Request(t, data, engine)
	})

	t.Run("time clock - start break success", func(t *testing.T) {
		data := map[string]interface{}{
			"cmd":               t1.CmdTimeClock,
			"version":           t1.CmdVersion,
			"device_sn":         "test_sn",
			"timestamp":         1607414000,
			"rfid":              "test_rfid",
			"clock_resp_status": 3,
		}

		// c := mock.NewMockThermal1Camera(ctrl)
		cc := mock.NewMockClient(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(1)

		testClockRecord := cloud.ClockRecord{
			TimeStamp:  1607414000,
			EmployeeID: "",
			RfId:       "test_rfid",
			Action:     "start_break",
		}

		box.EXPECT().CloudClient().Return(cc)
		cc.EXPECT().AddClockRecord(gomock.Eq(&testClockRecord)).Return(nil)
		// c.EXPECT().GetIP().Return(testIP).MinTimes(1)

		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			defer r.Body.Close()
			if assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				assert.Equal(t, map[string]interface{}{
					"version":      t1.CmdVersion,
					"cmd":          t1.CmdTimeClockResp,
					"clock_status": float64(3),
					"code":         float64(0),
				}, payload)
			}

			w.WriteHeader(http.StatusOK)
			w.Write(nil)
		}))

		var err error
		s.Listener, err = net.Listen("tcp", ":8000")
		require.NoError(t, err)

		s.Start()
		defer s.Close()

		sendThermal1Request(t, data, engine)
	})

	t.Run("time clock - start break failed", func(t *testing.T) {
		data := map[string]interface{}{
			"cmd":               t1.CmdTimeClock,
			"version":           t1.CmdVersion,
			"device_sn":         "test_sn",
			"timestamp":         1607414000,
			"rfid":              "test_rfid",
			"clock_resp_status": 3,
		}

		// c := mock.NewMockThermal1Camera(ctrl)
		cc := mock.NewMockClient(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(1)

		testClockRecord := cloud.ClockRecord{
			TimeStamp:  1607414000,
			EmployeeID: "",
			RfId:       "test_rfid",
			Action:     "start_break",
		}

		box.EXPECT().CloudClient().Return(cc)
		cc.EXPECT().AddClockRecord(gomock.Eq(&testClockRecord)).Return(errors.New("AddClockRecord ERROR"))
		// c.EXPECT().GetIP().Return(testIP).MinTimes(1)

		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			defer r.Body.Close()
			if assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				assert.Equal(t, map[string]interface{}{
					"version":      t1.CmdVersion,
					"cmd":          t1.CmdTimeClockResp,
					"clock_status": float64(3),
					"code":         float64(-1),
				}, payload)
			}

			w.WriteHeader(http.StatusOK)
			w.Write(nil)
		}))

		var err error
		s.Listener, err = net.Listen("tcp", ":8000")
		require.NoError(t, err)

		s.Start()
		defer s.Close()

		cg := camera2.NewCamGroup(0)
		cg.AddCamera(thermal_1.NewLiveCamera("test_sn", "localhost", ""))

		sendThermal1Request(t, data, engine)
	})

	t.Run("time clock - end break success", func(t *testing.T) {
		data := map[string]interface{}{
			"cmd":               t1.CmdTimeClock,
			"version":           t1.CmdVersion,
			"device_sn":         "test_sn",
			"timestamp":         1607414000,
			"rfid":              "test_rfid",
			"clock_resp_status": 4,
		}

		// c := mock.NewMockThermal1Camera(ctrl)
		cc := mock.NewMockClient(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(1)

		testClockRecord := cloud.ClockRecord{
			TimeStamp:  1607414000,
			EmployeeID: "",
			RfId:       "test_rfid",
			Action:     "end_break",
		}

		box.EXPECT().CloudClient().Return(cc)
		cc.EXPECT().AddClockRecord(gomock.Eq(&testClockRecord)).Return(nil)

		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			defer r.Body.Close()
			if assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				assert.Equal(t, map[string]interface{}{
					"version":      t1.CmdVersion,
					"cmd":          t1.CmdTimeClockResp,
					"clock_status": float64(4),
					"code":         float64(0),
				}, payload)
			}

			w.WriteHeader(http.StatusOK)
			w.Write(nil)
		}))

		var err error
		s.Listener, err = net.Listen("tcp", ":8000")
		require.NoError(t, err)

		s.Start()
		defer s.Close()

		cg := camera2.NewCamGroup(0)
		cg.AddCamera(thermal_1.NewLiveCamera("test_sn", "localhost", ""))

		sendThermal1Request(t, data, engine)
	})

	t.Run("time clock - end break failed", func(t *testing.T) {
		data := map[string]interface{}{
			"cmd":               t1.CmdTimeClock,
			"version":           t1.CmdVersion,
			"device_sn":         "test_sn",
			"timestamp":         1607414000,
			"rfid":              "test_rfid",
			"clock_resp_status": 4,
		}

		// c := mock.NewMockThermal1Camera(ctrl)
		cc := mock.NewMockClient(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(1)

		testClockRecord := cloud.ClockRecord{
			TimeStamp:  1607414000,
			EmployeeID: "",
			RfId:       "test_rfid",
			Action:     "end_break",
		}

		box.EXPECT().CloudClient().Return(cc)
		cc.EXPECT().AddClockRecord(gomock.Eq(&testClockRecord)).Return(errors.New("AddClockRecord ERROR"))

		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			defer r.Body.Close()
			if assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				assert.Equal(t, map[string]interface{}{
					"version":      t1.CmdVersion,
					"cmd":          t1.CmdTimeClockResp,
					"clock_status": float64(4),
					"code":         float64(-1),
				}, payload)
			}

			w.WriteHeader(http.StatusOK)
			w.Write(nil)
		}))

		var err error
		s.Listener, err = net.Listen("tcp", ":8000")
		require.NoError(t, err)

		s.Start()
		defer s.Close()

		cg := camera2.NewCamGroup(0)
		cg.AddCamera(thermal_1.NewLiveCamera("test_sn", "localhost", ""))

		sendThermal1Request(t, data, engine)
	})
}
