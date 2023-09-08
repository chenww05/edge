package thermal_1_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/codegangsta/inject"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/example/minibox/apis/common"
	"github.com/example/minibox/apis/structs"
	"github.com/example/minibox/apis/thermal_1"
	box2 "github.com/example/minibox/box"
	camera2 "github.com/example/minibox/camera"
	camera "github.com/example/minibox/camera/thermal_1"
	"github.com/example/minibox/cloud"
	"github.com/example/minibox/configs"
	"github.com/example/minibox/mock"
	"github.com/example/minibox/printer"
	"github.com/example/minibox/utils"
)

const (
	testPhoto       = "iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAFKElEQVRYhcWX608UZxTG919o+qHfqk1ra0xjYlubtdQqDY0x6iK6ChZ0FYT1Urx010W8Fos2TdAKWLW0VqxJFWLrwmI1tlguiY1WoyVURS7LzLwzoxixKAsoMr9+WGdAd61gjUxykknmXJ73nPOc845NVVUkSUKWZXRdR9M0hBCoqoqqqsiybH0TQhAMBtE0DUVR0HUdVVUtGyEEkiShKArBYND6HgwGURTF8mf6VlUVmyzLCCGQZdkyHGjc2trKicpT5OzIJyNrHfGpS4hLWkBc0gLsDid2h5MPE+cTl7SAaa7FuNeuJ2dHAccrf0d5cDgTnKqqKIpiiSzL2ILBICeranA/cD5/hZfvD5UgSRKlZQEcqRlWoKHKzDQ3h372I0kSh46WsXC1jxmL0knzrMX/y4lwBo6drCQmITHCeNrCxdZ7TKKL+NwCkg9U4K6sJ/O8zupLHXhb7uFtucfqSx1kntdxV9aTfKCC+NwCYhJdln1C2tII/xPi51DiL8eW6s3G7nAy5dNNJBeXM2NTnqU0ybUE15EqvE138UnGkMTb1IPrSBWTXO7wIebMZ07BQRK/KeWD+enYHU6cGcuxzViUzuTU5Xhb7rGmORwoNXAGx+cFZDeGqLhl0NwN6l2oC0FxW/SAWZLBwRsGf9yBc50QaDf4TDHwNIRw5h9gxYXreBq7WVV3k6W1jeEDOj/G5s7awFTfFjxX7jB7+z5m7diH52oXuUofbfeI+vzZCVkDgucoBq09kXpdfVB03cDXauA+dYmEL/cyb38ZnoYQdoeTpGWrsJ2orOK9hCQyfq1jVd1NfMH7+Fr7ojoc+ATa+wE0dD9er6cPtqn9up7Gbpz5P2B3ODl8tAybqqqUlgWInbcAR85OPjkjs+eaEeEoFArhSklmzOujyM7y0dEbTnuhHqlbV/cXkyfGMG7sm5T7/dTcBp9ksKTqCh9lrmXirCSKD5cihMBmcjRz0xbsDidpx85ScSvS6d7du3npxRcsqa6q4gvVoKw9Unf2zHhL77WRL9PU2Wv1lt3hJHNjDpqmoes6NkVREEKQsDhMlZUXbxCI4rRo756HANTWVLPtMQCcCTMtvVGvjKA5dB+fZLDyYht2h5NZ6Uv7B5EJIHZuCnaHE29TD19HKUFXVxdpi1yMHTOazRvXc+tBCfKjlODv+nriYifz7lvjOFYRoLoDi5p2h5PYuSnW2LapqooQwuK+2SwtT2hC355i3pkynfHTEij6KfBYve4+2Cr6m9CMo+t6OAPmDngUQK4wuBaVhgaVZ8/x6sgRlowZ/QaXW5ojNDv7YO+1h+fFQACKooRLEA2ATzJYL4dr3NANyl24EIJjJcXU5nnZmjqbqRPeZvr749m5bB61eV7KK/zU3oYzd+Bou8FmJXJgmXHMDWszUxENwKOS3dRD1XYftXneqHL8qw1PHNFmHHOd28xVPBgAa1p6WVhUwobCXRzfv4vqH4uoOfQt/u8KWVe4G9d+/6ABWPcBcw4MBoBPMvA09bD8nEbbPx1wvxfu93JZvc6yswJPU8+gAVgl0DQNSZIGDcCU85JmAfjtqjxoOzOOEKJ/ED2uCf9L0quvkuI/TYr/NEtOB4cMQNM0NE0LZ0DX9SEDeFoZ2ISKooRZoKrqcwdgNaF5ax0OAEIIbJIkoWnacwdgXtWtZfS8AZi/A0OeA8+yBJqmhf8LnoaG/xeAJEn9k3A4WGCyz2YuheGYA0KIMIDhYIHVA+ZAGA4aSpI0tPvAs86ALMv8CysqfF+oqf78AAAAAElFTkSuQmCC"
	testSN          = "012338-B8B07F-D29EEE"
	testIP          = "192.168.0.136"
	testEmail       = "test@admin.com"
	testVisitingWho = "test_buddy"
	testSite        = "test_site"
)

func newQuestionnaireData(sn string, temp float64) map[string]interface{} {
	return map[string]interface{}{
		"cap_time": "2020/09/05 04:27:21.051",
		"closeup_pic": map[string]interface{}{
			"data":        testPhoto,
			"face_height": 762,
			"face_width":  576,
			"face_x":      236,
			"face_y":      462,
			"format":      "png",
		},
		"closeup_pic_flag": true,
		"cmd":              thermal_1.CmdQuestionnaire,
		"device_sn":        sn,
		"is_realtime":      1,
		"match": map[string]interface{}{
			"format":        "jpg",
			"is_encryption": false,
			"long_card_id":  0,
			"match_type":    []string{"anyface"},
			"origin":        "none",
			"person_attr":   "none",
			"person_role":   camera.PersonRoleEmployee,
		},
		"match_failed_reson": -6,
		"match_result":       100,
		"overall_pic_flag":   false,
		"pass":               true,
		"person": map[string]interface{}{
			"age":          0,
			"face_quality": 98,
			"has_mask":     true,
			"hat":          "none",
			"rotate_angle": 0,
			"sex":          "",
			"temperature":  temp,
			"turn_angle":   -2,
		},
		"sequence_no":  7,
		"version":      thermal_1.CmdVersion,
		"video_flag":   false,
		"email":        testEmail,
		"site":         testSite,
		"visiting_who": testVisitingWho,
	}
}

func sendThermal1Request(t *testing.T, req interface{}, handler http.Handler) {
	b, err := json.Marshal(req)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	r, err := http.NewRequest("POST", "/", bytes.NewReader(b))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestScanFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gin.SetMode(gin.TestMode)
	engine := gin.Default()

	box := mock.NewMockBox(ctrl)

	injector := inject.New()
	injector.Map(common.ThermalBaseAPI{
		Box: box,
	})

	thermal_1.Register(injector, engine)

	t.Run("heartbeat", func(t *testing.T) {
		data := map[string]interface{}{
			"addr_name": "",
			"addr_no":   "",
			"cmd":       thermal_1.CmdHeartBeat,
			"device_no": "",
			"device_sn": testSN,
			"ip":        testIP,
			"version":   thermal_1.CmdVersion,
		}

		box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(nil, errors.New("camera not found"))
		box.EXPECT().AddCamera(gomock.Any())

		sendThermal1Request(t, data, engine)
	})

	t.Run("upload event", func(t *testing.T) {
		data := map[string]interface{}{
			"addr_name": "",
			"addr_no":   "",
			"cap_time":  "2020/09/05 04:27:21.051",
			"closeup_pic": map[string]interface{}{
				"data":        testPhoto,
				"face_height": 762,
				"face_width":  576,
				"face_x":      236,
				"face_y":      462,
				"format":      "png",
			},
			"closeup_pic_flag": true,
			"cmd":              thermal_1.CmdFace,
			"device_no":        "",
			"device_sn":        testSN,
			"is_realtime":      1,
			"match": map[string]interface{}{
				"customer_text": "",
				"format":        "jpg",
				"is_encryption": false,
				"long_card_id":  0,
				"match_type":    []string{"anyface"},
				"origin":        "none",
				"person_attr":   "none",
				"person_id":     "",
				"person_name":   "",
				"person_role":   1,
			},
			"match_failed_reson": -6,
			"match_result":       100,
			"overall_pic_flag":   false,
			"person": map[string]interface{}{
				"age":          0,
				"face_quality": 98,
				"has_mask":     true,
				"hat":          "none",
				"rotate_angle": 0,
				"sex":          "",
				"temperatur":   37.32,
				"turn_angle":   -2,
			},
			"sequence_no": 7,
			"version":     thermal_1.CmdVersion,
			"video_flag":  false,
		}

		c := mock.NewMockThermal1Camera(ctrl)
		cfg := mock.NewMockConfig(ctrl)
		p := printer.NewMockPrintStrategy(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(2)

		box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
		box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
		box.EXPECT().GetPrintStrategy().Return(p)

		c.EXPECT().GetIP().Return(testIP).MinTimes(1)
		c.EXPECT().GetID().Return(1)
		c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
		c.EXPECT().SendEventToWSConn(gomock.Eq(false), gomock.Eq(testPhoto), gomock.Eq(utils.Celsius(37.32)), gomock.Eq(utils.Celsius(37.5))).
			Do(func(isF, imgBase64, temperature, limit interface{}) { wg.Done() })
		c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
			Min:   35.0,
			Max:   38.0,
			Limit: 37.5,
		}, nil).MinTimes(1)

		cfg.EXPECT().ShouldScanRfid().Return(false)
		cfg.EXPECT().GetScanPeriodSecs().Return(0)
		cfg.EXPECT().GetTemperatureUnit().Return(configs.CUnit)
		cfg.EXPECT().GetVisitorConfig().Return(&configs.VisitorConfig{})
		cfg.EXPECT().GetDisableCloud().Return(true)

		cfg.EXPECT().GetEventSavedHours().Return(0)

		p.EXPECT().Print(gomock.Any(), gomock.Any()).Do(func(arg1, arg2 interface{}) { wg.Done() }).Return(nil)

		sendThermal1Request(t, data, engine)
		assert.False(t, utils.WaitTimeout(&wg, time.Second))
	})
}

func TestQuestionnaireFail(t *testing.T) {
	t.Run("heartbeat", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		gin.SetMode(gin.TestMode)
		engine := gin.Default()

		box := mock.NewMockBox(ctrl)
		store := mock.NewMockDBClient(ctrl)

		injector := inject.New()
		injector.Map(common.ThermalBaseAPI{
			Box: box,
			DB:  store,
		})

		thermal_1.Register(injector, engine)
		data := map[string]interface{}{
			"cmd":       thermal_1.CmdHeartBeat,
			"device_sn": testSN,
			"ip":        testIP,
			"version":   thermal_1.CmdVersion,
		}

		box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(nil, errors.New("camera not found"))
		box.EXPECT().AddCamera(gomock.Any())

		sendThermal1Request(t, data, engine)
	})

	t.Run("questionnaire fail", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		gin.SetMode(gin.TestMode)
		engine := gin.Default()

		box := mock.NewMockBox(ctrl)
		store := mock.NewMockDBClient(ctrl)

		injector := inject.New()
		injector.Map(common.ThermalBaseAPI{
			Box: box,
			DB:  store,
		})

		thermal_1.Register(injector, engine)

		data := map[string]interface{}{
			"cmd":            thermal_1.CmdQuestionnaire,
			"version":        thermal_1.CmdVersion,
			"device_sn":      testSN,
			"pass":           false,
			"person_name":    "test_person",
			"person_role":    1,
			"person_company": "test_company",
		}

		c := mock.NewMockThermal1Camera(ctrl)
		cfg := mock.NewMockConfig(ctrl)
		p := printer.NewMockPrintStrategy(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(1)

		box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
		box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
		box.EXPECT().GetPrintStrategy().Return(p)

		c.EXPECT().GetIP().Return(testIP).MinTimes(1)
		c.EXPECT().GetID().Return(1)
		c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
		c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
			Min:   35.0,
			Max:   38.0,
			Limit: 37.5,
		}, nil).MinTimes(1)

		// TODO it should scan RFID here
		// cfg.EXPECT().ShouldScanRfid().Return(false)
		cfg.EXPECT().GetEventSavedHours().Return(0)
		cfg.EXPECT().GetVisitorConfig().Return(&configs.VisitorConfig{})
		cfg.EXPECT().GetDisableCloud().Return(true)

		p.EXPECT().Print(gomock.Any(), gomock.Any()).Do(func(arg1, arg2 interface{}) { wg.Done() }).Return(nil)

		sendThermal1Request(t, data, engine)

		assert.False(t, utils.WaitTimeout(&wg, time.Second))
	})

	t.Run("questionnaire fail save to DB", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		gin.SetMode(gin.TestMode)
		engine := gin.Default()

		box := mock.NewMockBox(ctrl)
		store := mock.NewMockDBClient(ctrl)

		injector := inject.New()
		injector.Map(common.ThermalBaseAPI{
			Box: box,
			DB:  store,
		})

		thermal_1.Register(injector, engine)

		data := map[string]interface{}{
			"cmd":            thermal_1.CmdQuestionnaire,
			"version":        thermal_1.CmdVersion,
			"device_sn":      testSN,
			"pass":           false,
			"person_name":    "test_person",
			"person_role":    1,
			"person_company": "test_company",
		}

		c := mock.NewMockThermal1Camera(ctrl)
		cfg := mock.NewMockConfig(ctrl)
		p := printer.NewMockPrintStrategy(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(2)

		box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
		box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
		box.EXPECT().GetPrintStrategy().Return(p)

		c.EXPECT().GetIP().Return(testIP).MinTimes(1)
		c.EXPECT().GetID().Return(1)
		c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
		c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
			Min:   35.0,
			Max:   38.0,
			Limit: 37.5,
		}, nil).MinTimes(1)

		// TODO it should scan RFID here
		// cfg.EXPECT().ShouldScanRfid().Return(false)
		cfg.EXPECT().GetEventSavedHours().Return(1)
		cfg.EXPECT().GetVisitorConfig().Return(&configs.VisitorConfig{})
		cfg.EXPECT().GetDisableCloud().Return(true)

		p.EXPECT().Print(gomock.Any(), gomock.Any()).Do(func(arg1, arg2 interface{}) { wg.Done() }).Return(nil)

		store.EXPECT().SaveEventToDB(gomock.Eq(testSN), gomock.Eq(1), gomock.Any(), gomock.Nil(), gomock.Any()).
			Do(func(arg1, arg2, arg3, arg4, arg5 interface{}) { wg.Done() }).Return(uint(1), nil)

		sendThermal1Request(t, data, engine)

		assert.False(t, utils.WaitTimeout(&wg, time.Second))
	})

	t.Run("questionnaire fail upload save to DB", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		gin.SetMode(gin.TestMode)
		engine := gin.Default()

		box := mock.NewMockBox(ctrl)
		store := mock.NewMockDBClient(ctrl)

		injector := inject.New()
		injector.Map(common.ThermalBaseAPI{
			Box: box,
			DB:  store,
		})

		thermal_1.Register(injector, engine)

		data := map[string]interface{}{
			"cmd":            thermal_1.CmdQuestionnaire,
			"version":        thermal_1.CmdVersion,
			"device_sn":      testSN,
			"pass":           false,
			"person_name":    "test_person",
			"person_role":    1,
			"person_company": "test_company",
		}

		c := mock.NewMockThermal1Camera(ctrl)
		cfg := mock.NewMockConfig(ctrl)
		p := printer.NewMockPrintStrategy(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(3)

		box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
		box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
		box.EXPECT().GetPrintStrategy().Return(p)
		box.EXPECT().UploadCameraEvent(gomock.Eq(1), gomock.Nil(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(arg1, arg2, arg3, arg4, arg5, arg6 interface{}) { wg.Done() }).Return("", errors.New("fail"))

		c.EXPECT().GetIP().Return(testIP).MinTimes(1)
		c.EXPECT().GetID().Return(1)
		c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
		c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
			Min:   35.0,
			Max:   38.0,
			Limit: 37.5,
		}, nil).MinTimes(1)

		// TODO it should scan RFID here
		// cfg.EXPECT().ShouldScanRfid().Return(false)
		cfg.EXPECT().GetEventSavedHours().Return(0)
		cfg.EXPECT().GetVisitorConfig().Return(&configs.VisitorConfig{})
		cfg.EXPECT().GetDisableCloud().Return(false)

		p.EXPECT().Print(gomock.Any(), gomock.Any()).Do(func(arg1, arg2 interface{}) { wg.Done() }).Return(nil)

		store.EXPECT().SaveEventToDB(gomock.Eq(testSN), gomock.Eq(1), gomock.Any(), gomock.Nil(), gomock.Any()).
			Do(func(arg1, arg2, arg3, arg4, arg5 interface{}) { wg.Done() }).Return(uint(1), nil)

		sendThermal1Request(t, data, engine)

		assert.False(t, utils.WaitTimeout(&wg, time.Second))
	})
}

func TestQuestionnaireSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gin.SetMode(gin.TestMode)
	engine := gin.Default()

	box := mock.NewMockBox(ctrl)

	injector := inject.New()
	injector.Map(common.ThermalBaseAPI{
		Box: box,
	})

	thermal_1.Register(injector, engine)

	t.Run("heartbeat", func(t *testing.T) {
		data := map[string]interface{}{
			"cmd":       thermal_1.CmdHeartBeat,
			"device_sn": testSN,
			"ip":        testIP,
			"version":   thermal_1.CmdVersion,
		}

		box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(nil, errors.New("camera not found"))
		box.EXPECT().AddCamera(gomock.Any())

		sendThermal1Request(t, data, engine)
	})

	t.Run("questionnaire success", func(t *testing.T) {
		data := map[string]interface{}{
			"cap_time": "2020/09/05 04:27:21.051",
			"closeup_pic": map[string]interface{}{
				"data":        testPhoto,
				"face_height": 762,
				"face_width":  576,
				"face_x":      236,
				"face_y":      462,
				"format":      "png",
			},
			"closeup_pic_flag": true,
			"cmd":              thermal_1.CmdQuestionnaire,
			"device_sn":        testSN,
			"is_realtime":      1,
			"match": map[string]interface{}{
				"format":        "jpg",
				"is_encryption": false,
				"long_card_id":  0,
				"match_type":    []string{"anyface"},
				"origin":        "none",
				"person_attr":   "none",
				"person_role":   1,
			},
			"match_failed_reson": -6,
			"match_result":       100,
			"overall_pic_flag":   false,
			"pass":               true,
			"person": map[string]interface{}{
				"age":          0,
				"face_quality": 98,
				"has_mask":     true,
				"hat":          "none",
				"rotate_angle": 0,
				"sex":          "",
				"temperature":  37.32,
				"turn_angle":   -2,
			},
			"sequence_no": 7,
			"version":     thermal_1.CmdVersion,
			"video_flag":  false,
		}

		c := mock.NewMockThermal1Camera(ctrl)
		cfg := mock.NewMockConfig(ctrl)
		p := printer.NewMockPrintStrategy(ctrl)

		wg := sync.WaitGroup{}

		wg.Add(2)

		box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
		box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
		box.EXPECT().GetPrintStrategy().Return(p)

		c.EXPECT().GetIP().Return(testIP).MinTimes(1)
		c.EXPECT().GetID().Return(1)
		c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
		c.EXPECT().SendEventToWSConn(gomock.Eq(false), gomock.Eq(testPhoto), gomock.Eq(utils.Celsius(37.32)), gomock.Eq(utils.Celsius(37.5))).
			Do(func(arg1, arg2, arg3, arg4 interface{}) { wg.Done() })
		c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
			Min:   35.0,
			Max:   38.0,
			Limit: 37.5,
		}, nil).MinTimes(1)

		// TODO it should scan RFID here
		// cfg.EXPECT().ShouldScanRfid().Return(false)
		cfg.EXPECT().GetTemperatureUnit().Return(configs.CUnit)
		cfg.EXPECT().GetEventSavedHours().Return(0)
		cfg.EXPECT().GetVisitorConfig().Return(&configs.VisitorConfig{})
		cfg.EXPECT().GetDisableCloud().Return(true)

		p.EXPECT().Print(gomock.Any(), gomock.Any()).Do(func(arg1, arg2 interface{}) { wg.Done() }).Return(nil)

		sendThermal1Request(t, data, engine)

		assert.False(t, utils.WaitTimeout(&wg, time.Second))
	})
}

func TestSkipVisitorQuestionnaireFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gin.SetMode(gin.TestMode)
	engine := gin.Default()

	box := mock.NewMockBox(ctrl)

	injector := inject.New()
	injector.Map(common.ThermalBaseAPI{
		Box: box,
	})

	thermal_1.Register(injector, engine)

	t.Run("skip visitor questionnaire failed", func(t *testing.T) {
		data := map[string]interface{}{
			"cmd":         thermal_1.CmdQuestionnaire,
			"device_sn":   testSN,
			"person_role": 1,
			"pass":        false,
		}

		c := mock.NewMockThermal1Camera(ctrl)
		cfg := mock.NewMockConfig(ctrl)
		box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
		box.EXPECT().GetConfig().Return(cfg).MinTimes(1)

		c.EXPECT().GetIP().Return(testIP).Times(1)
		c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).Times(1)
		c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
			Min:   35.0,
			Max:   38.0,
			Limit: 37.5,
		}, nil).Times(1)

		v := &configs.VisitorConfig{
			DisableFailedQuestionnaire: true,
		}
		cfg.EXPECT().GetVisitorConfig().Return(v)
		sendThermal1Request(t, data, engine)
	})
}

func TestSkipVisitorTemperatureFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gin.SetMode(gin.TestMode)
	engine := gin.Default()

	box := mock.NewMockBox(ctrl)

	injector := inject.New()
	injector.Map(common.ThermalBaseAPI{
		Box: box,
	})

	thermal_1.Register(injector, engine)

	t.Run("skip visitor temperature failed", func(t *testing.T) {
		data := map[string]interface{}{
			"cmd":         thermal_1.CmdQuestionnaire,
			"device_sn":   testSN,
			"person_role": 1,
			"pass":        true,
			"match": map[string]interface{}{
				"person_role": 1,
			},
			"person": map[string]interface{}{
				"temperature": 37.6,
			},
		}

		c := mock.NewMockThermal1Camera(ctrl)
		cfg := mock.NewMockConfig(ctrl)
		box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
		box.EXPECT().GetConfig().Return(cfg).MinTimes(1)

		c.EXPECT().GetIP().Return(testIP).Times(1)
		c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).Times(1)
		c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
			Min:   35.0,
			Max:   38.0,
			Limit: 37.5,
		}, nil).Times(1)

		v := &configs.VisitorConfig{
			DisableFailedTemperature: true,
		}
		cfg.EXPECT().GetVisitorConfig().Return(v)
		sendThermal1Request(t, data, engine)
	})
}

func TestMultipleCamera(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gin.SetMode(gin.TestMode)
	engine := gin.Default()

	box := mock.NewMockBox(ctrl)

	injector := inject.New()
	injector.Map(common.ThermalBaseAPI{
		Box: box,
	})

	thermal_1.Register(injector, engine)

	data1 := camera.FaceInfo{
		Picture: &camera.Picture{
			Data:   testPhoto,
			Format: "png",
		},
		BaseReq: camera.BaseReq{
			Cmd:     thermal_1.CmdFace,
			SN:      testSN,
			Version: thermal_1.CmdVersion,
		},
		Person: &camera.Person{
			HasMask:     true,
			Temperature: 37.32,
		},
	}

	const sn2 = "sn2"
	data2 := camera.FaceInfo{
		Picture: &camera.Picture{
			Data:   testPhoto,
			Format: "png",
		},
		BaseReq: camera.BaseReq{
			Cmd:     thermal_1.CmdFace,
			SN:      sn2,
			Version: thermal_1.CmdVersion,
		},
		Person: &camera.Person{
			HasMask:     true,
			Temperature: 36.32,
		},
	}

	c := mock.NewMockThermal1Camera(ctrl)
	c2 := mock.NewMockThermal1Camera(ctrl)
	cfg := mock.NewMockConfig(ctrl)
	p := printer.NewMockPrintStrategy(ctrl)

	wg := sync.WaitGroup{}

	gomock.InOrder(
		box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1),
		box.EXPECT().GetCameraBySN(gomock.Eq(sn2)).Return(c2, nil).MinTimes(1),
	)

	box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
	box.EXPECT().GetPrintStrategy().Return(p).Times(2)

	c.EXPECT().GetIP().Return(testIP).MinTimes(1)
	c.EXPECT().GetID().Return(1)
	c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
	c.EXPECT().SendEventToWSConn(gomock.Eq(false), gomock.Eq(testPhoto), gomock.Eq(utils.Celsius(37.32)), gomock.Eq(utils.Celsius(37.5))).
		Do(func(arg1, arg2, arg3, arg4 interface{}) { wg.Done() })
	c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
		Min:   35.0,
		Max:   38.0,
		Limit: 37.5,
	}, nil).MinTimes(1)

	c2.EXPECT().GetIP().Return(testIP).MinTimes(1)
	c2.EXPECT().GetID().Return(2)
	c2.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
	c2.EXPECT().SendEventToWSConn(gomock.Eq(false), gomock.Eq(testPhoto), gomock.Eq(utils.Celsius(36.32)), gomock.Eq(utils.Celsius(37.6))).
		Do(func(arg1, arg2, arg3, arg4 interface{}) { wg.Done() })
	c2.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
		Min:   35.0,
		Max:   38.0,
		Limit: 37.6,
	}, nil).MinTimes(1)

	cfg.EXPECT().ShouldScanRfid().Return(false).Times(2)
	cfg.EXPECT().GetScanPeriodSecs().Return(0).Times(2)
	cfg.EXPECT().GetTemperatureUnit().Return(configs.CUnit).Times(2)
	cfg.EXPECT().GetDisableCloud().Return(true).Times(2)

	cfg.EXPECT().GetEventSavedHours().Return(0).Times(2)
	cfg.EXPECT().GetVisitorConfig().Return(&configs.VisitorConfig{}).Times(2)

	p.EXPECT().Print(gomock.Any(), gomock.Any()).Do(func(arg1, arg2 interface{}) { wg.Done() }).Return(nil).Times(2)

	// First request
	wg.Add(2)
	sendThermal1Request(t, data1, engine)
	assert.False(t, utils.WaitTimeout(&wg, time.Second))

	// Second request
	wg.Add(2)
	sendThermal1Request(t, data2, engine)
	assert.False(t, utils.WaitTimeout(&wg, time.Second))
}

func TestSaveEventUploadFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gin.SetMode(gin.TestMode)
	engine := gin.Default()

	box := mock.NewMockBox(ctrl)

	store := mock.NewMockDBClient(ctrl)

	injector := inject.New()
	injector.Map(common.ThermalBaseAPI{
		Logger: zerolog.New(zerolog.NewConsoleWriter()),
		Box:    box,
		DB:     store,
	})

	thermal_1.Register(injector, engine)

	data := newQuestionnaireData(testSN, 37.32)

	c := mock.NewMockThermal1Camera(ctrl)
	cfg := mock.NewMockConfig(ctrl)
	p := printer.NewMockPrintStrategy(ctrl)

	wg := sync.WaitGroup{}

	wg.Add(3)

	box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
	box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
	box.EXPECT().GetPrintStrategy().Return(p)
	box.EXPECT().UploadS3ByTokenName(gomock.Eq(1), gomock.Any(), gomock.Eq(762), gomock.Eq(576), gomock.Eq("jpg"), box2.TokenNameCameraEvent).Return(nil, errors.New("err"))

	c.EXPECT().GetIP().Return(testIP).MinTimes(1)
	c.EXPECT().GetID().Return(1).MinTimes(1)
	c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
	c.EXPECT().SendEventToWSConn(gomock.Eq(false), gomock.Eq(testPhoto), gomock.Eq(utils.Celsius(37.32)), gomock.Eq(utils.Celsius(37.5))).
		Do(func(arg1, arg2, arg3, arg4 interface{}) { wg.Done() })
	c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
		Min:   35.0,
		Max:   38.0,
		Limit: 37.5,
	}, nil).MinTimes(1)

	// TODO it should scan RFID here
	// cfg.EXPECT().ShouldScanRfid().Return(false)
	dir, _ := ioutil.TempDir("", "data")

	cfg.EXPECT().GetTemperatureUnit().Return(configs.CUnit)
	cfg.EXPECT().GetEventSavedHours().Return(0)
	cfg.EXPECT().GetVisitorConfig().Return(&configs.VisitorConfig{})
	cfg.EXPECT().GetDisableCloud().Return(false)
	cfg.EXPECT().GetDataStoreDir().Return(dir)

	p.EXPECT().Print(gomock.Any(), gomock.Any()).Do(func(arg1, arg2 interface{}) { wg.Done() }).Return(nil)

	store.EXPECT().SaveEventToDB(gomock.Eq(testSN), gomock.Eq(1), gomock.Eq(false), gomock.Any(), gomock.Any()).DoAndReturn(
		func(arg1, arg2, arg3, arg4, arg5 interface{}) (uint, error) {
			wg.Done()
			return 0, nil
		})

	sendThermal1Request(t, data, engine)

	assert.False(t, utils.WaitTimeout(&wg, time.Second))
}

func TestSaveEventUnknownCamera(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gin.SetMode(gin.TestMode)
	engine := gin.Default()

	box := mock.NewMockBox(ctrl)

	store := mock.NewMockDBClient(ctrl)

	injector := inject.New()
	injector.Map(common.ThermalBaseAPI{
		Logger: zerolog.New(zerolog.NewConsoleWriter()),
		Box:    box,
		DB:     store,
	})

	thermal_1.Register(injector, engine)

	data := newQuestionnaireData(testSN, 37.32)

	c := mock.NewMockThermal1Camera(ctrl)
	cfg := mock.NewMockConfig(ctrl)
	p := printer.NewMockPrintStrategy(ctrl)

	wg := sync.WaitGroup{}

	wg.Add(3)

	box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
	box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
	box.EXPECT().GetPrintStrategy().Return(p)

	c.EXPECT().GetIP().Return(testIP).MinTimes(1)
	c.EXPECT().GetID().Return(0).MinTimes(1)
	c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
	c.EXPECT().SendEventToWSConn(gomock.Eq(false), gomock.Eq(testPhoto), gomock.Eq(utils.Celsius(37.32)), gomock.Eq(utils.Celsius(37.5))).
		Do(func(arg1, arg2, arg3, arg4 interface{}) { wg.Done() })
	c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
		Min:   35.0,
		Max:   38.0,
		Limit: 37.5,
	}, nil).MinTimes(1)

	cfg.EXPECT().GetTemperatureUnit().Return(configs.CUnit)
	cfg.EXPECT().GetEventSavedHours().Return(0)
	cfg.EXPECT().GetVisitorConfig().Return(&configs.VisitorConfig{})

	p.EXPECT().Print(gomock.Any(), gomock.Any()).Do(func(arg1, arg2 interface{}) { wg.Done() }).Return(nil)

	store.EXPECT().SaveEventToDB(gomock.Eq(testSN), gomock.Eq(0), gomock.Eq(false), gomock.Any(), gomock.Any()).DoAndReturn(
		func(arg1, arg2, arg3, arg4, arg5 interface{}) (uint, error) {
			wg.Done()
			return 0, nil
		})

	sendThermal1Request(t, data, engine)

	assert.False(t, utils.WaitTimeout(&wg, time.Second))
}

func TestDontSaveEventCloudPartialFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gin.SetMode(gin.TestMode)
	engine := gin.Default()

	box := mock.NewMockBox(ctrl)

	store := mock.NewMockDBClient(ctrl)

	injector := inject.New()
	injector.Map(common.ThermalBaseAPI{
		Logger: zerolog.New(zerolog.NewConsoleWriter()),
		Box:    box,
		DB:     store,
	})

	thermal_1.Register(injector, engine)

	data := newQuestionnaireData(testSN, 37.32)

	c := mock.NewMockThermal1Camera(ctrl)
	cfg := mock.NewMockConfig(ctrl)
	p := printer.NewMockPrintStrategy(ctrl)

	wg := sync.WaitGroup{}

	wg.Add(3)

	box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
	box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
	box.EXPECT().GetPrintStrategy().Return(p)
	box.EXPECT().UploadS3ByTokenName(gomock.Eq(1), gomock.Any(), gomock.Eq(762), gomock.Eq(576), gomock.Eq("jpg"), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil)
	box.EXPECT().UploadCameraEvent(gomock.Eq(1), gomock.Any(), gomock.Eq(37.32), gomock.Eq(37.5), gomock.Any(), gomock.Any()).
		DoAndReturn(func(arg1, arg2, arg3, arg4, arg5, arg6 interface{}) (int64, error) {
			wg.Done()
			return 10, cloud.Err{Code: 202}
		})

	c.EXPECT().GetIP().Return(testIP).MinTimes(1)
	c.EXPECT().GetID().Return(1).MinTimes(1)
	c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
	c.EXPECT().SendEventToWSConn(gomock.Eq(false), gomock.Eq(testPhoto), gomock.Eq(utils.Celsius(37.32)), gomock.Eq(utils.Celsius(37.5))).
		Do(func(arg1, arg2, arg3, arg4 interface{}) { wg.Done() })
	c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
		Min:   35.0,
		Max:   38.0,
		Limit: 37.5,
	}, nil).MinTimes(1)

	// TODO it should scan RFID here
	// cfg.EXPECT().ShouldScanRfid().Return(false)
	dir, _ := ioutil.TempDir("", "data")

	cfg.EXPECT().GetTemperatureUnit().Return(configs.CUnit)
	cfg.EXPECT().GetEventSavedHours().Return(0)
	cfg.EXPECT().GetVisitorConfig().Return(&configs.VisitorConfig{})
	cfg.EXPECT().GetDisableCloud().Return(false)
	cfg.EXPECT().GetDataStoreDir().Return(dir)

	p.EXPECT().Print(gomock.Any(), gomock.Any()).Do(func(arg1, arg2 interface{}) { wg.Done() }).Return(nil)

	sendThermal1Request(t, data, engine)

	assert.False(t, utils.WaitTimeout(&wg, time.Second))
}

func TestDontSaveEventCloudFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gin.SetMode(gin.TestMode)
	engine := gin.Default()

	box := mock.NewMockBox(ctrl)

	store := mock.NewMockDBClient(ctrl)

	injector := inject.New()
	injector.Map(common.ThermalBaseAPI{
		Logger: zerolog.New(zerolog.NewConsoleWriter()),
		Box:    box,
		DB:     store,
	})

	thermal_1.Register(injector, engine)

	data := newQuestionnaireData(testSN, 37.32)

	c := mock.NewMockThermal1Camera(ctrl)
	cfg := mock.NewMockConfig(ctrl)
	p := printer.NewMockPrintStrategy(ctrl)

	wg := sync.WaitGroup{}

	wg.Add(3)

	box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
	box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
	box.EXPECT().GetPrintStrategy().Return(p)
	box.EXPECT().UploadS3ByTokenName(gomock.Eq(1), gomock.Any(), gomock.Eq(762), gomock.Eq(576), gomock.Eq("jpg"), box2.TokenNameCameraEvent).Return(&utils.S3File{}, nil)
	box.EXPECT().UploadCameraEvent(gomock.Eq(1), gomock.Any(), gomock.Eq(37.32), gomock.Eq(37.5), gomock.Any(), gomock.Any()).
		DoAndReturn(func(arg1, arg2, arg3, arg4, arg5, arg6 interface{}) (int64, error) {
			wg.Done()
			return 0, cloud.Err{Code: 101}
		})

	c.EXPECT().GetIP().Return(testIP).MinTimes(1)
	c.EXPECT().GetID().Return(1).MinTimes(1)
	c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
	c.EXPECT().SendEventToWSConn(gomock.Eq(false), gomock.Eq(testPhoto), gomock.Eq(utils.Celsius(37.32)), gomock.Eq(utils.Celsius(37.5))).
		Do(func(arg1, arg2, arg3, arg4 interface{}) { wg.Done() })
	c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
		Min:   35.0,
		Max:   38.0,
		Limit: 37.5,
	}, nil).MinTimes(1)

	// TODO it should scan RFID here
	// cfg.EXPECT().ShouldScanRfid().Return(false)
	dir, _ := ioutil.TempDir("", "data")

	cfg.EXPECT().GetTemperatureUnit().Return(configs.CUnit)
	cfg.EXPECT().GetEventSavedHours().Return(0)
	cfg.EXPECT().GetVisitorConfig().Return(&configs.VisitorConfig{})
	cfg.EXPECT().GetDisableCloud().Return(false)
	cfg.EXPECT().GetDataStoreDir().Return(dir)

	p.EXPECT().Print(gomock.Any(), gomock.Any()).Do(func(arg1, arg2 interface{}) { wg.Done() }).Return(nil)

	sendThermal1Request(t, data, engine)

	assert.False(t, utils.WaitTimeout(&wg, time.Second))
}

type metaMatcher struct {
	Temperature float64
	Abnormal    float64
	Site        string
	Email       string
	VisitingWho string
	PersonRole  camera.PersonRole
}

func (m metaMatcher) String() string {
	return fmt.Sprintf("temp: %f, limit: %f", m.Temperature, m.Abnormal)
}

func (m metaMatcher) Matches(x interface{}) bool {
	if s, ok := x.(*structs.MetaScanData); ok {
		return m.Temperature == s.Temperature &&
			m.Abnormal == s.Abnormal &&
			m.Site == s.Site &&
			m.Email == s.Email &&
			m.PersonRole == s.PersonRole &&
			m.VisitingWho == s.VisitingWho
	}

	return false
}

func TestMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gin.SetMode(gin.TestMode)
	engine := gin.Default()

	box := mock.NewMockBox(ctrl)

	store := mock.NewMockDBClient(ctrl)

	injector := inject.New()
	injector.Map(common.ThermalBaseAPI{
		Logger: zerolog.New(zerolog.NewConsoleWriter()),
		Box:    box,
		DB:     store,
	})

	thermal_1.Register(injector, engine)

	data := newQuestionnaireData(testSN, 37.32)

	c := mock.NewMockThermal1Camera(ctrl)
	cfg := mock.NewMockConfig(ctrl)
	p := printer.NewMockPrintStrategy(ctrl)

	wg := sync.WaitGroup{}

	wg.Add(3)

	box.EXPECT().GetCameraBySN(gomock.Eq(testSN)).Return(c, nil).MinTimes(1)
	box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
	box.EXPECT().GetPrintStrategy().Return(p)

	c.EXPECT().GetIP().Return(testIP).MinTimes(1)
	c.EXPECT().GetID().Return(1).MinTimes(1)
	c.EXPECT().Heartbeat(gomock.Eq(testIP), gomock.Eq("")).MinTimes(1)
	c.EXPECT().SendEventToWSConn(gomock.Eq(false), gomock.Eq(testPhoto), gomock.Eq(utils.Celsius(37.32)), gomock.Eq(utils.Fahrenheit(90.0).ToC())).
		Do(func(arg1, arg2, arg3, arg4 interface{}) { wg.Done() })
	c.EXPECT().GetTemperatureConfig().Return(&camera.TemperatureConfig{
		Min:            80.0,
		Max:            100.0,
		Limit:          90.0,
		FahrenheitUnit: true,
	}, nil).MinTimes(1)

	cfg.EXPECT().GetTemperatureUnit().Return(configs.CUnit)
	cfg.EXPECT().GetEventSavedHours().Return(1)
	cfg.EXPECT().GetVisitorConfig().Return(&configs.VisitorConfig{})
	cfg.EXPECT().GetDisableCloud().Return(true)

	store.EXPECT().SaveEventToDB(gomock.Eq(testSN), gomock.Eq(1), gomock.Eq(true), gomock.Any(),
		metaMatcher{
			Temperature: 37.32,
			Abnormal:    utils.FtoC(90),
			Site:        testSite,
			Email:       testEmail,
			PersonRole:  camera.PersonRoleEmployee,
			VisitingWho: testVisitingWho,
		}).DoAndReturn(
		func(arg1, arg2, arg3, arg4, arg5 interface{}) (uint, error) {
			wg.Done()
			return 1, nil
		})

	p.EXPECT().Print(gomock.Any(), gomock.Any()).Do(func(arg1, arg2 interface{}) { wg.Done() }).Return(nil)

	sendThermal1Request(t, data, engine)

	assert.False(t, utils.WaitTimeout(&wg, time.Second))
}

func TestRFID(t *testing.T) {
	t.Run("does not allow two command strings", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		gin.SetMode(gin.TestMode)
		engine := gin.Default()

		box := mock.NewMockBox(ctrl)

		injector := inject.New()
		injector.Map(common.ThermalBaseAPI{
			Box: box,
		})

		thermal_1.Register(injector, engine)

		sendThermal1Request(t, json.RawMessage(`{"cmd": "ping card reader", "cmd": "fake"}`), engine)
	})

	t.Run("sends to all scanners", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		gin.SetMode(gin.TestMode)
		engine := gin.Default()

		box := mock.NewMockBox(ctrl)
		cfg := mock.NewMockConfig(ctrl)
		box.EXPECT().GetConfig().Return(cfg).MinTimes(1)
		cfg.EXPECT().GetTimeKeepingEnable().Return(false)

		injector := inject.New()
		injector.Map(common.ThermalBaseAPI{
			Box: box,
		})

		thermal_1.Register(injector, engine)

		data := map[string]interface{}{
			"cmd":       thermal_1.CmdCardReader,
			"test_data": "test_value",
		}
		count := 0

		s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count++
			var payload map[string]interface{}
			defer r.Body.Close()
			if assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
				assert.Equal(t, data, payload)
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
		cg.AddCamera(camera.NewLiveCamera("test_sn", "localhost", ""))
		cg.AddCamera(camera.NewLiveCamera("test_sn2", "localhost", ""))

		box.EXPECT().GetCamGroup().Return(cg)

		sendThermal1Request(t, data, engine)
		assert.Equal(t, 2, count)
	})
}
