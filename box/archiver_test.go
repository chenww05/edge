package box

import (
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/example/minibox/configs"
	"github.com/example/minibox/db"
	"github.com/example/minibox/mock"
)

func Test_NewArchiveTask(t *testing.T) {
	// TODO
}
func Test_StopArchiveTask(t *testing.T) {
	// TODO
}

func Test_UpdateArchiveTask(t *testing.T) {
	// TODO
}

func Test_FindMissingRanges(t *testing.T) {

	//cfg := configs.NewEmptyConfig()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := mock.NewMockBox(ctrl)
	mdb := mock.NewMockDBClient(ctrl)

	var config configs.CloudStorageConfig
	atr := NewArchiveTaskRunner(b, mdb, config)

	videos := []db.VideoStartEnd{
		{StartTime: 1652163368, EndTime: 1652163427}, {StartTime: 1652163487, EndTime: 1652163547}, {StartTime: 1652163547, EndTime: 1652163607},
		//{StartTime: 1652163607, EndTime: 1652163667}, {StartTime: 1652163667, EndTime: 1652163727}, {StartTime: 1652163727, EndTime: 1652163787},
		//{StartTime: 1652163787, EndTime: 1652163847}, {StartTime: 1652163847, EndTime: 1652163907}, {StartTime: 1652163907, EndTime: 1652163967},
		//{StartTime: 1652163967, EndTime: 1652164027},{StartTime:1652164027 ,EndTime:1652164087},{StartTime:1652164087, EndTime:1652164147},
		//{StartTime:1652164147, EndTime:1652164207},{StartTime:1652164207, EndTime:1652164267},{StartTime:1652164267, EndTime:1652164327},
		//{StartTime:1652164327 EndTime:1652164387}{StartTime:1652164387 EndTime:1652164447}{StartTime:1652164447 EndTime:1652164507}{StartTime:1652164507 EndTime:1652164567}{StartTime:1652164567 EndTime:1652164587}{StartTime:1652164587 EndTime:1652164647}{StartTime:1652164647 EndTime:1652164707}{StartTime:1652164707 EndTime:1652164767}{StartTime:1652164767 EndTime:1652164827}{StartTime:1652164827 EndTime:1652164887}{StartTime:1652164887 EndTime:1652164947}{StartTime:1652164947 EndTime:1652165007}{StartTime:1652165007 EndTime:1652165067}{StartTime:1652165067 EndTime:1652165127}{StartTime:1652165127 EndTime:1652165187}{StartTime:1652165187 EndTime:1652165247}{StartTime:1652165247 EndTime:1652165307}{StartTime:1652165307 EndTime:1652165368}{StartTime:1652165368 EndTime:1652165428}{StartTime:1652165428 EndTime:1652165488}{StartTime:1652165488 EndTime:1652165548}{StartTime:1652166863 EndTime:1652166923}{StartTime:1652166923 EndTime:1652166983}{StartTime:1652166983 EndTime:1652167043}]
	}
	ret := atr.findMissingRanges(videos, 1652162368, 1652167043)
	t.Logf("%+v", ret)

	problems := []db.VideoStartEnd{
		{StartTime: 1652170944, EndTime: 1652171003}, {StartTime: 1652171003, EndTime: 1652171063}, {StartTime: 1652171063, EndTime: 1652171123},
		{StartTime: 1652171123, EndTime: 1652171183}, {StartTime: 1652171183, EndTime: 1652171243}, {StartTime: 1652171243, EndTime: 1652171303},
		{StartTime: 1652171303, EndTime: 1652171363}, {StartTime: 1652171363, EndTime: 1652171423}, {StartTime: 1652171363, EndTime: 1652171363},
		{StartTime: 1652171363, EndTime: 1652171423}, {StartTime: 1652171423, EndTime: 1652171483}, {StartTime: 1652171423, EndTime: 1652171423},
		{StartTime: 1652171423, EndTime: 1652171483}, {StartTime: 1652171423, EndTime: 1652171423}, {StartTime: 1652171483, EndTime: 1652171543},
		{StartTime: 1652171483, EndTime: 1652171483}, {StartTime: 1652171543, EndTime: 1652171603}, {StartTime: 1652171603, EndTime: 1652171663},
		{StartTime: 1652171663, EndTime: 1652171723}, {StartTime: 1652171723, EndTime: 1652171783}, {StartTime: 1652171783, EndTime: 1652171843},
		{StartTime: 1652171843, EndTime: 1652171903}, {StartTime: 1652171903, EndTime: 1652171963}, {StartTime: 1652171963, EndTime: 1652172023},
	}

	ret = atr.findMissingRanges(problems, 1652162368, 1652167043)
	t.Logf("%+v", ret)
}
