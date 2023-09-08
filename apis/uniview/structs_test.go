package uniview

import "testing"

func TestObjectInfo_GetPersonListObject(t *testing.T) {
	o := ObjectInfo{PersonInfoList: []PersonInfo{
		{ObjectDetected: ObjectDetected{}},
		{ObjectDetected: ObjectDetected{}},
	}}
	o.GetPersonListObject()
}

func TestObjectInfo_GetVehicleListObject(t *testing.T) {
	o := ObjectInfo{VehicleInfoList: []VehicleInfo{
		{ObjectDetected: ObjectDetected{}},
		{ObjectDetected: ObjectDetected{}},
	}}
	o.GetVehicleListObject()
}

func TestObjectInfo_d_GetMotorCycleListObject(t *testing.T) {
	o := ObjectInfo{NonMotorVehicleInfoList: []NonMotorVehicleInfo{
		{ObjectDetected: ObjectDetected{}},
		{ObjectDetected: ObjectDetected{}},
	}}
	o.GetMotorCycleListObject()
}
