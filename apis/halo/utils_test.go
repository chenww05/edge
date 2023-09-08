package halo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/turingvideo/minibox/cloud"
)

func Test_getHaloEventType(t *testing.T) {
	type args struct {
		event string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "getVapeEvent",
			args: args{
				event: "Vape",
			},
			want: cloud.HaloEventVape,
		},
		{
			name: "getAmmoniaEvent",
			args: args{
				event: "NH3",
			},
			want: cloud.HaloEventAmmonia,
		},
		{
			name: "getEmptyEvent",
			args: args{
				event: "",
			},
			want: cloud.HaloEventUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, getHaloEventType(tt.args.event), "GetHaloEventType(%v)", tt.args.event)
		})
	}
}

func Test_createSensorList(t *testing.T) {
	type args struct {
		sensors map[string]float64
		active  []string
	}
	tests := []struct {
		name string
		args args
		want []cloud.HaloSensorStatus
	}{
		{
			name: "createEmptyList",
			args: args{
				sensors: map[string]float64{},
				active:  []string{},
			},
			want: []cloud.HaloSensorStatus(nil),
		},
		{
			name: "createList",
			args: args{
				sensors: map[string]float64{
					"Light":    float64(1.23),
					"Humidity": float64(123),
				},
				active: []string{"Light"},
			},
			want: []cloud.HaloSensorStatus{
				{EventType: "light", Value: float64(1.23), IsActive: true},
				{EventType: "humidity", Value: float64(123), IsActive: false},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, createSensorList(tt.args.sensors, tt.args.active), "CreateSensorList(%v)", tt.args.sensors, tt.args.active)
		})
	}
}