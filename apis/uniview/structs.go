package uniview

type AlarmInfo struct {
	AlarmSrcType int    `json:"AlarmSrcType"`
	AlarmSrcID   int    `json:"AlarmSrcID"`
	Timestamp    int64  `json:"TimeStamp"`
	AlarmType    string `json:"AlarmType"`
}

type AlarmNotification struct {
	Reference string    `json:"Reference"`
	AlarmInfo AlarmInfo `json:"AlarmInfo"`
}

type Point struct {
	X int `json:"X"`
	Y int `json:"Y"`
}

type RuleInfo struct {
	RuleType    uint32  `json:"RuleType"`
	TriggerType uint32  `json:"TriggerType"`
	PointNum    int64   `json:"PointNum"`
	PointList   []Point `json:"PointList"`
}

type ObjectDetectedInfo interface {
	GetPosition() string
	GetConfidence() uint32
	GetLargePicIndex() uint32
}

type ObjectDetected struct {
	Position            string `json:"Position"`
	Confidence          uint32 `json:"Confidence"`
	SmallPicAttachIndex uint32 `json:"SmallPicAttachIndex"`
	LargePicAttachIndex uint32 `json:"LargePicAttachIndex"`
}

func (o ObjectDetected) GetPosition() string {
	return o.Position
}

func (o ObjectDetected) GetConfidence() uint32 {
	return o.Confidence
}

func (o ObjectDetected) GetLargePicIndex() uint32 {
	return o.LargePicAttachIndex
}

type PersonInfo struct {
	ObjectDetected
	PersonID          uint32     `json:"PersonID"`
	PersonDoforFaceID uint32     `json:"PersonDoforFaceId"`
	AppearTime        string     `json:"AppearTime"`
	DisAppearTime     string     `json:"DisAppearTime"`
	FeatureVersion    string     `json:"FeatureVersion"`
	Feature           string     `json:"Feature"`
	AttributeInfo     PersonAttr `json:"AttributeInfo"`
	RuleInfo          RuleInfo   `json:"RuleInfo"`
}

type FaceAttr struct {
	Gender           uint32  `json:"Gender,omitempty"`
	AgeRange         uint32  `json:"AgeRange,omitempty"`
	EthicCode        uint32  `json:"EthicCode,omitempty"`
	GlassFlag        uint32  `json:"GlassFlag,omitempty"`
	GlassesStyle     uint32  `json:"GlassesStyle,omitempty"`
	GlassesColor     uint32  `json:"GlassesColor,omitempty"`
	HairStyle        uint32  `json:"HairStyle,omitempty"`
	HairColor        uint32  `json:"HairColor,omitempty"`
	FaceStyle        uint32  `json:"FaceStyle,omitempty"`
	SkinColor        uint32  `json:"SkinColor,omitempty"`
	EyebrowStyle     uint32  `json:"EyebrowStyle,omitempty"`
	WrinklePouch     uint32  `json:"WrinklePouch,omitempty"`
	NoseStyle        uint32  `json:"NoseStyle,omitempty"`
	Beard            uint32  `json:"Beard,omitempty"`
	LipStyle         uint32  `json:"LipStyle,omitempty"`
	MustacheStyle    uint32  `json:"MustacheStyle,omitempty"`
	MaskFlag         uint32  `json:"MaskFlag,omitempty"`
	MaskColor        uint32  `json:"MaskColor,omitempty"`
	HatFlag          uint32  `json:"HatFlag,omitempty"`
	HatStyle         uint32  `json:"HatStyle,omitempty"`
	HatColor         uint32  `json:"HatColor,omitempty"`
	Scarf            uint32  `json:"Scarf,omitempty"`
	ScarfColor       uint32  `json:"ScarfColor,omitempty"`
	CoatColor        uint32  `json:"CoatColor,omitempty"`
	AcneStain        uint32  `json:"AcneStain,omitempty"`
	FreckleBirthmark uint32  `json:"FreckleBirthmark,omitempty"`
	ScarDimple       uint32  `json:"ScarDimple,omitempty"`
	FacialFeature    uint32  `json:"FacialFeature,omitempty"`
	Temperature      float32 `json:"Temperature,omitempty"`
}

type FaceInfo struct {
	FaceID                     uint32   `json:"FaceID"`
	FaceDoforPersonID          uint32   `json:"FaceDoforPersonID"`
	FaceDoforNonMotorVehicleID uint32   `json:"FaceDoforNonMotorVehicleID"`
	FaceDoforVehicleID         uint32   `json:"FaceDoforVehicleID"`
	Position                   string   `json:"Position"`
	AppearTime                 string   `json:"AppearTime"`
	DisAppearTime              string   `json:"DisAppearTime"`
	Confidence                 uint32   `json:"Confidence"`
	SmallPicAttachIndex        uint32   `json:"SmallPicAttachIndex"`
	LargePicAttachIndex        uint32   `json:"LargePicAttachIndex"`
	FeatureVersion             string   `json:"FeatureVersion"`
	Feature                    string   `json:"Feature"`
	AttributeInfo              FaceAttr `json:"AttributeInfo"`
}

// NonMotorVehicleAttr is referred in document, bu not defined
type NonMotorVehicleAttr struct {
}

// PlateAttr is referred in document, bu not defined
type PlateAttr struct {
}

// PersonAttr is referred in document, bu not defined
type PersonAttr struct {
}

type NonMotorVehicleInfo struct {
	ObjectDetected
	ID                        uint32              `json:"ID,omitempty"`
	PlatePicAttachIndex       uint32              `json:"PlatePicAttachIndex,omitempty"`
	AppearTime                string              `json:"AppearTime,omitempty"`
	DisAppearTime             string              `json:"DisAppearTime,omitempty"`
	Speed                     uint32              `json:"Speed,omitempty"`
	DirectionType             string              `json:"DirectionType,omitempty"`
	AttributeInfo             NonMotorVehicleAttr `json:"AttributeInfo"`
	PlateAttributeInfo        PlateAttr           `json:"PlateAttributeInfo"`
	PersonOnNoVehicleNum      uint32              `json:"PersonOnNoVehicleNum,omitempty"`
	PersonOnNoVehicleInfoList PersonAttr          `json:"PersonOnNoVehicleInfoList"`
	RuleInfo                  RuleInfo            `json:"RuleInfo"`
}

// VehicleAttr is referred in document, bu not defined
type VehicleAttr struct {
}

// VehicleFaceInfo is referred in document, bu not defined
type VehicleFaceInfo struct {
}

// GpsInfo is referred in document, bu not defined
type GpsInfo struct {
}

// EVIInfo is referred in document, bu not defined
type EVIInfo struct {
}

type VehicleInfo struct {
	ObjectDetected
	ID                   uint32            `json:"ID,omitempty"`
	PlatePicAttachIndex  uint32            `json:"PlatePicAttachIndex,omitempty"`
	AppearTime           string            `json:"AppearTime,omitempty"`
	DisAppearTime        string            `json:"DisAppearTime,omitempty"`
	TriggerType          uint32            `json:"TriggerType,omitempty"`
	FeatureVersion       string            `json:"FeatureVersion,omitempty"`
	Feature              string            `json:"Feature,omitempty"`
	VehicleAttributeInfo []VehicleAttr     `json:"VehicleAttributeInfo,omitempty"`
	PlateAttributeInfo   []PlateAttr       `json:"PlateAttributeInfo,omitempty"`
	VehicleFaceInfo      []VehicleFaceInfo `json:"VehicleFaceInfo,omitempty"`
	GpsInfo              []GpsInfo         `json:"GpsInfo,omitempty"`
	EVIInfo              []EVIInfo         `json:"EviInfo,omitempty"`
	RuleInfo             RuleInfo          `json:"RuleInfo,omitempty"`
}

type ObjectInfo struct {
	FaceNum                 uint32                `json:"FaceNum"`
	FaceInfoList            []FaceInfo            `json:"FaceInfoList"`
	PersonNum               uint32                `json:"PersonNum"`
	PersonInfoList          []PersonInfo          `json:"PersonInfoList"`
	NonMotorVehicleNum      uint32                `json:"NonMotorVehicleNum"`
	NonMotorVehicleInfoList []NonMotorVehicleInfo `json:"NonMotorVehicleInfoList"`
	VehicleNum              uint32                `json:"VehicleNum"`
	VehicleInfoList         []VehicleInfo         `json:"VehicleInfoList"`
}

func (o ObjectInfo) GetPersonListObject() []ObjectDetected {
	objects := make([]ObjectDetected, 0)
	for _, item := range o.PersonInfoList {
		objects = append(objects, item.ObjectDetected)
	}
	return objects
}

func (o ObjectInfo) GetVehicleListObject() []ObjectDetected {
	objects := make([]ObjectDetected, 0)
	for _, item := range o.VehicleInfoList {
		objects = append(objects, item.ObjectDetected)
	}
	return objects
}

func (o ObjectInfo) GetMotorCycleListObject() []ObjectDetected {
	objects := make([]ObjectDetected, 0)
	for _, item := range o.NonMotorVehicleInfoList {
		objects = append(objects, item.ObjectDetected)
	}
	return objects
}

type ImageInfo struct {
	Index        uint32 `json:"Index"`
	Type         uint32 `json:"Type"`
	Format       uint32 `json:"Format"`
	Width        uint32 `json:"Width"`
	Height       uint32 `json:"Height"`
	CaptureTime  uint32 `json:"CaptureTime"`
	DataType     uint32 `json:"DataType"`
	Size         uint32 `json:"Size"`
	Data         string `json:"Data"`
	URL          string `json:"URL"`
	UploadID     string `json:"UploadId"`
	UploadStatus uint32 `json:"UploadStatus"`
	ErrorCode    uint32 `json:"ErrorCode"`
	TransferTime uint32 `json:"TransferTime"`
}

type StructureDataInfo struct {
	ObjInfo        ObjectInfo  `json:"ObjInfo"`
	ImageNum       uint32      `json:"ImageNum"`
	ImageInfoList  []ImageInfo `json:"ImageInfoList"`
	FinishFaceNum  uint32      `json:"FinishFaceNum"`
	FinishFaceList []uint32    `json:"FinishFaceList"`
}

type EventNotification struct {
	Reference        string            `json:"Reference"`
	Timestamp        int64             `json:"TimeStamp"`
	Seq              uint32            `json:"Seq"`
	SrcID            uint32            `json:"SrcID"`
	SrcName          string            `json:"SrcName"`
	NotificationType uint32            `json:"NotificationType"`
	DeviceID         string            `json:"DeviceID"`
	RelatedID        string            `json:"RelatedID"`
	StructureInfo    StructureDataInfo `json:"StructureInfo"`
}

type MotionDetectionAlarm struct {
	Reference    string       `json:"Reference"`
	AlarmType    string       `json:"AlarmType"`
	Timestamp    int64        `json:"TimeStamp"`
	Seq          uint32       `json:"Seq"`
	SourceID     uint32       `json:"SourceID"`
	AlarmPicture AlarmPicture `json:"AlarmPicture"`
}

type AlarmPicture struct {
	ImageNum  uint32      `json:"ImageNum"`
	ImageList []ImageInfo `json:"ImageList"`
}

const (
	EventEnterArea = "EnterArea"
	EventLeaveArea = "LeaveArea"
	EventIntrusion = "FieldDetectorObjectsInside"
	EventCrossLine = "LineDetectorCrossed"
	EventFace      = "ObjectIsRecognized"

	EventPeople     = "person"
	EventCar        = "car"
	EventMotorcycle = "motorcycle"
	EventBoat       = "boat"
	EventBus        = "bus"
	EventTruck      = "truck"
	EventTrain      = "train"
)
