package database

import (
	"log"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Enums
type ReferralStatus string

const (
	Created          ReferralStatus = "Created"
	Consented        ReferralStatus = "Consented"
	Complete         ReferralStatus = "Complete"
	Granted          ReferralStatus = "Granted"
	UploadIncomplete ReferralStatus = "UploadIncomplete"
	UploadComplete   ReferralStatus = "UploadComplete"

	NotGranted ReferralStatus = "NotGranted"
)

type UploadStatus string

const (
	IncompleteUpload UploadStatus = "UploadIncomplete"
	CompleteUpload   UploadStatus = "UploadComplete"
)

type ChunkStatus = string

const (
	IncompleteChunk ChunkStatus = "Incomplete"
	CompleteChunk   ChunkStatus = "Complete"
)

type Chunk struct {
	Checksum string `json:"Checksum" validate:"required"`
	SizeKB   int    `json:"SizeKB" validate:"required"` // round up
	Status   ChunkStatus
}

type ChunkFile struct {
	Name   string  `json:"Name" validate:"required"` // Identify file by name
	Chunks []Chunk `json:"Chunks" validate:"required,dive"`
}

// Pseudo Types
type CreationData struct {
	Diagnosis string `json:"Diagnosis" validate:"required"`
	History   string `json:"History" validate:"required"`
}
type PatientObject struct {
	CitizenId string `json:"CitizenId" validate:"required"`
	Prefix    string `json:"Prefix" validate:"required"`
	FirstName string `json:"FirstName" validate:"required"`
	LastName  string `json:"LastName" validate:"required"`
	BirthDate string `json:"BirthDate" validate:"required,datetime=2006-01-02"`
	Address   string `json:"Address" validate:"required"`
	Gender    string `json:"Gender" validate:"required"`
	Telephone string `json:"Telephone" validate:"required"`
	Email     string `json:"Email" validate:"required,email"`
}

type ReferralObject struct {
	Origin      string `json:"Origin" validate:"required"`
	Destination string `json:"Destination" validate:"required"`
	Department  string `json:"Department" validate:"required"`
	Reason      string `json:"Reason" validate:"required"`
}

type FileObject struct {
	Name     string `json:"Name" validate:"required"`
	Checksum string `json:"Checksum" validate:"required"`
}

// DB Types
type Database struct {
	database *gorm.DB
}

type Referral struct {
	Id int `gorm:"primaryKey;autoIncrement"`
	ReferralObject
	OriginHospital      Hospital `gorm:"foreignKey:Origin;references:HospitalId"`
	DestinationHospital Hospital `gorm:"foreignKey:Destination;references:HospitalId"`
	PatientObject
	ReferralStatus ReferralStatus
	Created        int64  `gorm:"autoCreateTime"`
	PayloadKey     string `gorm:"payloadKey"`
}

// Like a receipt for outgoing referrals
type ReferralReceipt struct {
	Id       int `gorm:"primaryKey;autoIncrement"`
	Referral int
	DoctorId string
}

type Patient struct {
	Id         int `gorm:"primaryKey;autoIncrement"`
	Username   string
	Password   string
	IsVerified bool
	CitizenId  string
}

type Hospital struct {
	Id           int    `gorm:"primaryKey;autoIncrement"`
	HospitalId   string `json:"HospitalId" validate:"required"`
	HospitalName string `json:"HospitalName" validate:"required"`
	CertSerial   string
}

type File struct {
	Id            int `gorm:"primaryKey;autoIncrement"`
	Referral      int
	ReferralModel Referral `gorm:"foreignKey:Referral;references:Id"`
	ParentPath    string
	UploadStatus  UploadStatus
	FileObject
}

// Database Management

func fillTestData(db *gorm.DB) {
	h1 := Hospital{
		Id:           1,
		HospitalId:   "1111",
		HospitalName: "First Government Hospital",
		CertSerial:   "342359423506035269845572243484938265229640821055",
	}
	db.Save(&h1)

	h2 := Hospital{
		Id:           2,
		HospitalId:   "2222",
		HospitalName: "Second Private Hospital",
		CertSerial:   "342359423506035269845572243484938265229640821056",
	}
	db.Save(&h2)

	h3 := Hospital{
		Id:           3,
		HospitalId:   "3333",
		HospitalName: "Third Military Hospital",
		CertSerial:   "a",
	}
	db.Save(&h3)
}

func NewDatabase(dbname string) (database Database) {
	if err := os.MkdirAll(filepath.Dir(dbname), 0770); err != nil {
		log.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(dbname), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	db.AutoMigrate(&File{})
	db.AutoMigrate(&Referral{})
	db.AutoMigrate(&ReferralReceipt{})
	db.AutoMigrate(&Hospital{})
	db.AutoMigrate(&Patient{})
	// db.AutoMigrate(&ClientAccount{})

	fillTestData(db)

	return Database{database: db}
}

func (db *Database) CreateReferralServer(referral Referral) (id int, ok bool) {
	referral.ReferralStatus = Created
	result := db.database.Omit("Id").Create(&referral)
	if result.Error != nil {
		return 0, false
	}
	return referral.Id, true
}
func (db *Database) CreatePatient(patient Patient) (id int, ok bool) {
	// referral.ReferralStatus = Created
	result := db.database.Omit("Id").Create(&patient)
	if result.Error != nil {
		return 0, false
	}
	return patient.Id, true
}

func (db *Database) GetPatientByUsername(username string) (r Patient, ok bool) {
	result := db.database.Where("username = ?", username).First(&r)
	if result.Error != nil {
		return r, false
	}
	return r, true
}

func (db *Database) GetPatientByCitizenId(citizen_id string) (r Patient, ok bool) {
	result := db.database.Where("citizen_id = ?", citizen_id).First(&r)
	if result.Error != nil {
		return r, false
	}
	return r, true
}

func (db *Database) GetPatientByUsernamePassword(username string, password string) (r Patient, ok bool) {
	result := db.database.Where("username = ? AND password = ?", username, password).First(&r)
	if result.Error != nil {
		return r, false
	}
	return r, true
}

func (db *Database) CreateReferralReceipt(rec ReferralReceipt) (err error) {
	result := db.database.Omit("Id").Create(&rec)
	return result.Error
}

func (db *Database) GetReceiptByReferral(referralId int) (r ReferralReceipt, ok bool) {
	result := db.database.Where("referral = ?", referralId).First(&r)
	if result.Error != nil {
		return r, false
	}
	return r, true
}

func (db *Database) GetReferralsByDestination(hospitalId string) (r []Referral) {
	db.database.Where("destination = ?", hospitalId).Find(&r)
	return
}

func (db *Database) GetReferralsByOrigin(hospitalId string) (r []Referral) {
	db.database.Where("origin = ?", hospitalId).Find(&r)
	return
}

func (db *Database) GetReceiptByDoctor(doctorId string) (r []ReferralReceipt) {
	db.database.Where("doctor_id = ?", doctorId).Find(&r)
	return
}

func (db *Database) GetReferralById(id int) (r Referral, ok bool) {
	r = Referral{
		Id: id,
	}
	result := db.database.First(&r)
	// Note: pure first with no where can only be done
	// 	when using PK
	if result.Error != nil {
		return r, false
	}
	return r, true
}

func (db *Database) UpdateStatusReferralById(id int, status ReferralStatus) (ok bool) {
	result := db.database.Model(&Referral{Id: id}).Update("referral_status", status)
	if result.RowsAffected == 0 {
		return false
	}
	return result.Error == nil
}

func (db *Database) ServerCreateFile(file File) (id int, ok bool) {
	result := db.database.Omit("Id").Create(&file)
	if result.Error != nil {
		return 0, false
	}
	return file.Id, true
}

func (db *Database) ClientCreateFile(file File) (id int, ok bool) {
	result := db.database.Omit("Id").Create(&file)
	if result.Error != nil {
		return 0, false
	}
	return file.Id, true
}

func (db *Database) GetFilesByReferral(id int) (fs []File, ok bool) {
	result := db.database.Where("referral = ?", id).Find(&fs)
	if result.Error != nil {
		return fs, false
	}
	return fs, true
}

func (db *Database) GetFileByReferralName(referralId int, name string) (fs File, ok bool) {
	result := db.database.Where("referral = ? and name = ?", referralId, name).First(&fs)
	if result.Error != nil {
		return fs, false
	}
	return fs, true
}

func (db *Database) UpdateStatusFileById(id int, status UploadStatus) (ok bool) {
	result := db.database.Model(&File{Id: id}).Update("upload_status", status)
	if result.RowsAffected == 0 {
		return false
	}
	return result.Error == nil
}

func (db *Database) UpdatePayloadKeyById(id int, payloadKey string) (ok bool) {
	result := db.database.Model(&Referral{Id: id}).Update("payload_key", payloadKey)
	if result.RowsAffected == 0 {
		return false
	}
	return result.Error == nil
}

func (db *Database) ServerCreateHospital(hos Hospital) (id int, ok bool) {
	result := db.database.Omit("Id").Create(&hos)
	if result.Error != nil {
		return 0, false
	}
	return hos.Id, true
}

func (db *Database) GetHospitalBySerial(serial string) (hos Hospital, ok bool) {
	result := db.database.Where("cert_serial = ?", serial).First(&hos)
	if result.Error != nil {
		return hos, false
	}
	return hos, true
}

func (db *Database) GetHospitals() (hos []Hospital, ok bool) {
	result := db.database.Find(&hos)
	if result.RowsAffected == 0 {
		return hos, false
	}
	return hos, true
}

func (db *Database) GetReferralsByPatient(citizenId string) (r []Referral) {
	db.database.Where("citizen_id = ?", citizenId).Find(&r)
	return
}

func FilestoMap(fileArray []File) (fileMap map[string]File) {
	fileMap = make(map[string]File)
	for _, item := range fileArray {
		fileMap[item.Name] = item
	}
	return fileMap
}
