package common_test

import (
	"testing"

	"github.com/example/minibox/apis/common"
)

func TestIsDataURI(t *testing.T) {
	var (
		test1 = "data:image/png;base64,/983t74utjir"
		test2 = "data:,hejkowkle"
		test3 = "data:image/jpg;charset=UTF-8;base64,/eugijoksej,euhji"
		tests = []string{test1, test2, test3}
	)

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			if !common.IsDataURI(test) {
				t.Error(test, "Is not data uri")
			}
		})
	}

	var (
		testNo = []string{"data/aijwiwok", "data;image/png;base64,ajwioefja"}
	)

	for _, test := range testNo {
		t.Run(test, func(t *testing.T) {
			if common.IsDataURI(test) {
				t.Error(test, "Is data uri")
			}
		})
	}
}

func TestGetDataURIData(t *testing.T) {
	tests := []struct {
		uri  string
		data string
	}{{"data:image/png;base64,123", "123"}, {"data:,/9d,24", "/9d,24"}, {"data:,", ""}, {"data:iawufweuh", ""}}

	for _, test := range tests {
		t.Run(test.uri, func(t *testing.T) {
			if val := common.GetDataURIData(test.uri); val != test.data {
				t.Error(val, " != ", test.data)
			}
		})
	}
}
