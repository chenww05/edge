package halo

type HaloEventNotification struct {
	Name        string  `json:"name"`
	Timestamp   string  `json:"timestamp"`
	MAC         string  `json:"mac"`
	EventType   string  `json:"event_type"`
	Threshold   float64 `json:"threshold"`
	SensorValue float64 `json:"sensor_value"`
	DataSource  string  `json:"data_source"`
}

type HaloHeartbeat struct {
	Name      string             `json:"name"`
	Timestamp string             `json:"timestamp"`
	MAC       string             `json:"mac"`
	Sensors   map[string]float64 `json:"sensors"`
	Active    string             `json:"active"`
}

