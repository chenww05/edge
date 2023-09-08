package thermal_1

import (
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/example/minibox/apis/common"
	"github.com/example/minibox/camera/thermal_1"
	"github.com/example/minibox/mock"
)

func TestGetAndClean(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	box := mock.NewMockBox(ctrl)
	cache := NewFaceCache(common.ThermalBaseAPI{
		Box: box,
	})

	t.Run("call before add", func(t *testing.T) {
		f := cache.GetAndClean()

		assert.Equal(t, thermal_1.FaceInfo{}, f)
		assert.Equal(t, 0, cache.counter)
		assert.Nil(t, cache.data)
	})

	t.Run("returns copy and clears data", func(t *testing.T) {
		initial := thermal_1.FaceInfo{
			Name: "test_name",
			Picture: &thermal_1.Picture{
				Data: "test_data",
			},
			Person: &thermal_1.Person{
				Temperature: 35.0,
			},
			QrCode: &thermal_1.QrCode{
				QrData: "test_qr_data",
			},
		}

		cache.data = &initial
		f := cache.GetAndClean()

		assert.NotSame(t, f.Picture, initial.Picture)
		assert.NotSame(t, f.Person, initial.Person)
		assert.NotSame(t, f.QrCode, initial.QrCode)

		assert.Equal(t, f, initial)
		assert.Equal(t, 0, cache.counter)
		assert.Nil(t, cache.data)
	})
}

func TestAdd(t *testing.T) {
	t.Run("sets data in cache", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		box := mock.NewMockBox(ctrl)

		cache := NewFaceCache(common.ThermalBaseAPI{
			Box: box,
		})

		initial := thermal_1.FaceInfo{
			Name: "test_name",
		}

		c := mock.NewMockConfig(ctrl)
		box.EXPECT().GetConfig().Return(c)
		c.EXPECT().GetScanPeriodSecs().Return(5)
		c.EXPECT().ShouldScanRfid().Return(false)

		cache.Add(&initial)

		if assert.NotNil(t, cache.data) {
			assert.Equal(t, initial, *cache.data)
		}

		assert.Equal(t, 1, cache.counter)
	})

	t.Run("parallel add calls", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		box := mock.NewMockBox(ctrl)

		cache := NewFaceCache(common.ThermalBaseAPI{
			Box: box,
		})

		initial := thermal_1.FaceInfo{
			BaseReq: thermal_1.BaseReq{
				Version: "test_version",
			},
			Name: "test_name",
		}

		c := mock.NewMockConfig(ctrl)

		box.EXPECT().GetConfig().Return(c).Times(3)

		c.EXPECT().GetScanPeriodSecs().Return(5)
		c.EXPECT().ShouldScanRfid().Return(false).Times(3)

		wg := sync.WaitGroup{}
		wg.Add(2)

		cache.Add(&initial)

		go func() {
			cache.Add(&thermal_1.FaceInfo{
				Picture: &thermal_1.Picture{
					Data: "test_picture",
				},
			})
			wg.Done()
		}()

		go func() {
			cache.Add(&thermal_1.FaceInfo{
				QrCode: &thermal_1.QrCode{
					QrData: "test_data",
				},
			})
			wg.Done()
		}()

		wg.Wait()

		if assert.NotNil(t, cache.data) {
			assert.Equal(t, thermal_1.FaceInfo{
				Name: "test_name",
				BaseReq: thermal_1.BaseReq{
					Version: "test_version",
				},
				Picture: &thermal_1.Picture{
					Data: "test_picture",
				},
				QrCode: &thermal_1.QrCode{
					QrData: "test_data",
				},
			}, *cache.data)
		}
		assert.Equal(t, 3, cache.counter)
	})

	t.Run("multiple add calls", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		box := mock.NewMockBox(ctrl)

		cache := NewFaceCache(common.ThermalBaseAPI{
			Box: box,
		})

		initial := thermal_1.FaceInfo{
			BaseReq: thermal_1.BaseReq{
				Version: "test_version",
			},
			Name: "test_name",
		}

		c := mock.NewMockConfig(ctrl)
		tq := mock.NewMockThermal1Camera(ctrl)

		box.EXPECT().GetConfig().Return(c).Times(4)
		box.EXPECT().GetCameraBySN(gomock.Eq("")).Return(tq, nil)

		c.EXPECT().GetScanPeriodSecs().Return(5)
		c.EXPECT().ShouldScanRfid().Return(false).Times(4)

		tq.EXPECT().GetTemperatureConfig().Return(&thermal_1.TemperatureConfig{
			Min: 30,
			Max: 40,
		}, nil)

		cache.Add(&initial)

		assert.NotNil(t, cache.data)
		assert.Equal(t, 1, cache.counter)

		cache.Add(&thermal_1.FaceInfo{
			QrCode: &thermal_1.QrCode{
				QrData: "test_data",
			},
		})

		assert.NotNil(t, cache.data)
		assert.Equal(t, 2, cache.counter)

		cache.Add(&thermal_1.FaceInfo{
			Picture: &thermal_1.Picture{
				Data: "test_picture",
			},
		})

		assert.NotNil(t, cache.data)
		assert.Equal(t, 3, cache.counter)

		cache.Add(&thermal_1.FaceInfo{
			Person: &thermal_1.Person{
				Temperature: 35.0,
			},
		})

		if assert.NotNil(t, cache.data) {
			assert.Equal(t, thermal_1.FaceInfo{
				Name: "test_name",
				BaseReq: thermal_1.BaseReq{
					Version: "test_version",
				},
				Picture: &thermal_1.Picture{
					Data: "test_picture",
				},
				QrCode: &thermal_1.QrCode{
					QrData: "test_data",
				},
				Person: &thermal_1.Person{
					Temperature: 35.0,
				},
			}, *cache.data)
		}

		assert.Equal(t, 4, cache.counter)
	})
}

func TestTemperatureAveraging(t *testing.T) {
	t.Run("gets the average of temperatures", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		box := mock.NewMockBox(ctrl)

		cache := NewFaceCache(common.ThermalBaseAPI{
			Box: box,
		})

		c := mock.NewMockConfig(ctrl)
		tq := mock.NewMockThermal1Camera(ctrl)

		box.EXPECT().GetConfig().Return(c).Times(2)
		box.EXPECT().GetCameraBySN(gomock.Eq("")).Return(tq, nil).Times(2)

		c.EXPECT().GetScanPeriodSecs().Return(5)
		c.EXPECT().ShouldScanRfid().Return(false).Times(2)

		tq.EXPECT().GetTemperatureConfig().Return(&thermal_1.TemperatureConfig{
			Min: 20,
			Max: 50,
		}, nil).Times(2)

		cache.Add(&thermal_1.FaceInfo{
			Name: "test_name",
			Person: &thermal_1.Person{
				Temperature: 30.0,
			},
		})

		cache.Add(&thermal_1.FaceInfo{
			Name: "test_name",
			Person: &thermal_1.Person{
				Temperature: 40.0,
			},
		})

		data := cache.GetAndClean()
		assert.Equal(t, 35.0, data.Person.Temperature)
	})

	t.Run("doesn't average when the temperature is zero", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		box := mock.NewMockBox(ctrl)

		cache := NewFaceCache(common.ThermalBaseAPI{
			Box: box,
		})

		c := mock.NewMockConfig(ctrl)
		tq := mock.NewMockThermal1Camera(ctrl)

		box.EXPECT().GetConfig().Return(c).Times(11)
		box.EXPECT().GetCameraBySN(gomock.Eq("")).Return(tq, nil).MinTimes(4)

		c.EXPECT().GetScanPeriodSecs().Return(5).Times(4)
		c.EXPECT().ShouldScanRfid().Return(false).Times(11)

		tq.EXPECT().GetTemperatureConfig().Return(&thermal_1.TemperatureConfig{
			Min: 20,
			Max: 50,
		}, nil).MinTimes(4)

		cache.Add(&thermal_1.FaceInfo{
			Name: "test_name",
			Person: &thermal_1.Person{
				Temperature: 30.0,
			},
		})

		cache.Add(&thermal_1.FaceInfo{
			Person: &thermal_1.Person{
				Temperature: 0.0,
			},
		})

		data := cache.GetAndClean()
		assert.Equal(t, 30.0, data.Person.Temperature)

		cache.Add(&thermal_1.FaceInfo{
			Name: "test_name",
			Person: &thermal_1.Person{
				Temperature: 0.0,
			},
		})

		cache.Add(&thermal_1.FaceInfo{
			Person: &thermal_1.Person{
				Temperature: 30.0,
			},
		})

		data = cache.GetAndClean()
		assert.Equal(t, 30.0, data.Person.Temperature)

		cache.Add(&thermal_1.FaceInfo{
			Name: "test_name",
			Person: &thermal_1.Person{
				Temperature: 0.0,
			},
		})

		cache.Add(&thermal_1.FaceInfo{
			Person: &thermal_1.Person{
				Temperature: 30.0,
			},
		})

		cache.Add(&thermal_1.FaceInfo{
			Person: &thermal_1.Person{
				Temperature: 0.0,
			},
		})

		data = cache.GetAndClean()
		assert.Equal(t, 30.0, data.Person.Temperature)

		cache.Add(&thermal_1.FaceInfo{
			Name: "test_name",
			Person: &thermal_1.Person{
				Temperature: 0.0,
			},
		})

		cache.Add(&thermal_1.FaceInfo{
			Person: &thermal_1.Person{
				Temperature: 30.0,
			},
		})

		cache.Add(&thermal_1.FaceInfo{
			Person: &thermal_1.Person{
				Temperature: 0.0,
			},
		})

		cache.Add(&thermal_1.FaceInfo{
			Person: &thermal_1.Person{
				Temperature: 40.0,
			},
		})

		data = cache.GetAndClean()
		assert.Equal(t, 35.0, data.Person.Temperature)
	})

	t.Run("doesn't add for temp out of range", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		box := mock.NewMockBox(ctrl)

		cache := NewFaceCache(common.ThermalBaseAPI{
			Box: box,
		})

		c := mock.NewMockConfig(ctrl)
		tq := mock.NewMockThermal1Camera(ctrl)

		box.EXPECT().GetConfig().Return(c).Times(4)
		box.EXPECT().GetCameraBySN(gomock.Eq("")).Return(tq, nil).Times(4)

		c.EXPECT().GetScanPeriodSecs().Return(5).Times(2)
		c.EXPECT().ShouldScanRfid().Return(false).Times(4)

		tq.EXPECT().GetTemperatureConfig().Return(&thermal_1.TemperatureConfig{
			Min: 20,
			Max: 50,
		}, nil).Times(4)

		cache.Add(&thermal_1.FaceInfo{
			Name: "test_name",
			Person: &thermal_1.Person{
				Temperature: 30.0,
			},
		})

		cache.Add(&thermal_1.FaceInfo{
			Person: &thermal_1.Person{
				Temperature: 10.0,
			},
		})

		data := cache.GetAndClean()
		assert.Equal(t, 30.0, data.Person.Temperature)

		cache.Add(&thermal_1.FaceInfo{
			Name: "test_name",
			Person: &thermal_1.Person{
				Temperature: 10.0,
			},
		})

		cache.Add(&thermal_1.FaceInfo{
			Person: &thermal_1.Person{
				Temperature: 30.0,
			},
		})

		data = cache.GetAndClean()
		assert.Equal(t, 30.0, data.Person.Temperature)
	})
}
