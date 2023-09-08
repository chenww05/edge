package halo

import "github.com/turingvideo/minibox/cloud"

const (
	Aggression      = "Aggression"
	AirQualityIndex = "AQI"
	CarbonMonoxide  = "CO"
	CarbonDioxide   = "CO2cal"
	Gunshot         = "Gunshot"
	HealthIndex     = "Health_Index"
	Help            = "Help"
	Humidity        = "Humidity"
	Light           = "Light"
	Masking         = "Masking"
	Ammonia         = "NH3"
	NitrogenDioxide = "NO2"
	PM1             = "PM1"
	PM10            = "PM10"
	PM25            = "PM2.5"
	Pressure        = "Pressure"
	Sound           = "Sound"
	Temperature     = "Temp_F"
	THC             = "THC"
	TVOC            = "TVOC"
	Vape            = "Vape"
	Tamper          = "Tamper"
)

func getHaloEventType(event string) string {
	switch event {
	case Aggression:
		event = cloud.HaloEventAggression
	case AirQualityIndex:
		event = cloud.HaloEventAirQualityIndex
	case CarbonMonoxide:
		event = cloud.HaloEventCarbonMonoxide
	case CarbonDioxide:
		event = cloud.HaloEventCarbonDioxide
	case Gunshot:
		event = cloud.HaloEventGunshot
	case HealthIndex:
		event = cloud.HaloEventHealthIndex
	case Help:
		event = cloud.HaloEventHelp
	case Humidity:
		event = cloud.HaloEventHumidity
	case Light:
		event = cloud.HaloEventLight
	case Masking:
		event = cloud.HaloEventMasking
	case Ammonia:
		event = cloud.HaloEventAmmonia
	case NitrogenDioxide:
		event = cloud.HaloEventNitrogenDioxide
	case PM1:
		event = cloud.HaloEventPM1
	case PM10:
		event = cloud.HaloEventPM10
	case PM25:
		event = cloud.HaloEventPM25
	case Pressure:
		event = cloud.HaloEventPressure
	case Sound:
		event = cloud.HaloEventSound
	case Temperature:
		event = cloud.HaloEventTemperature
	case THC:
		event = cloud.HaloEventTHC
	case TVOC:
		event = cloud.HaloEventTVOC
	case Vape:
		event = cloud.HaloEventVape
	case Tamper:
		event = cloud.HaloEventTamper
	default:
		return cloud.HaloEventUnknown
	}
	return event
}

func isActive(val string, list []string) bool {
	for _, v := range list {
		if v == val {
			return true
		}
	}
	return false
}

func createSensorList(list map[string]float64, activeList []string) []cloud.HaloSensorStatus {
	var sensors []cloud.HaloSensorStatus

	for name, val := range list {
		sensors = append(sensors, cloud.HaloSensorStatus{
			EventType: getHaloEventType(name),
			Value:     val,
			IsActive:  isActive(name, activeList),
		})
	}
	return sensors
}
