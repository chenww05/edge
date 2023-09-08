package ppl_2

import (
	"fmt"
	"testing"
	"time"
)

func Test_ExpTime(t *testing.T) {
	timeStr := time.Now().Format("2006-01-02")
	t2, _ := time.ParseInLocation("2006-01-02", timeStr, time.Local)
	dur := time.Duration(t2.AddDate(0, 0, 1).Unix() - time.Now().Unix())
	if dur > 24*time.Minute {
		t.Error("dur is error")
	}
	//fmt.Println(dur)
	startTime := time.Now().UTC().UnixNano() / 1000000
	fmt.Println(startTime)
	fmt.Println(time.Unix(1647832790828, 0))
}
