package hishandler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"simplemts/lib"
	db "simplemts/lib/database"
	"slices"
	"strings"
)

const (
	HN_INIT      = 3744
	MAX_PATIENTS = 30 // max patients in HIS
)

type HISPatient struct {
	Hn  string `json:"Hn" validate:"required"`
	PID string
	db.PatientObject
}

type His struct {
	dir string
}

func InitiateConnection() His {
	hisDir := lib.GetEnv("HIS_DIR", "../../his")
	his := His{
		dir: hisDir,
	}
	// his.parseFiles()
	return his
}

func convertPrefix(pfx string) string {
	switch pfx {
	case "Mr.":
		return "mr"
	case "Mrs.":
		return "mrs"
	case "Ms.":
		return "ms"
	default:
		return "mr" // No "none"
	}
}
func convertGender(gender string) string {
	switch gender {
	case "M":
		return "male"
	case "F":
		return "female"
	default:
		return "male" // No "none"
	}
}

func (his *His) parsePatients() (patients []HISPatient) {
	f, err := os.Open(path.Join(his.dir, "csv/patients.csv"))
	if err != nil {
		log.Fatal(err)
	}
	csvReader := csv.NewReader(f)
	data, err := csvReader.ReadAll()
	f.Close()
	if err != nil {
		log.Fatal(err)
	}
	hn_counter := 0
	for i, line := range data {
		if i <= 0 { // omit header
			continue
		}
		bdate := strings.Split(line[1], "-")
		hn_counter += 1
		rec := HISPatient{
			PID: line[0],
			Hn:  fmt.Sprint(hn_counter + HN_INIT),
			PatientObject: db.PatientObject{
				CitizenId: line[3],
				Prefix:    convertPrefix(line[6]),
				FirstName: line[7],
				LastName:  line[8],
				BirthDate: line[1],
				Address:   fmt.Sprintf("%s, %s, %s, %s", line[16], line[17], line[18], line[19]),
				Gender:    convertGender(line[14]),
				Email:     fmt.Sprint(strings.ToLower(line[8]), "@email.com"),
				Telephone: fmt.Sprintf("09%s%s%s", bdate[1], bdate[2], bdate[0]),
			},
		}
		patients = append(patients, rec)
		if hn_counter > MAX_PATIENTS {
			break
		}
	}
	return
}

func (his *His) GetPatients() []HISPatient {
	return his.parsePatients()
}

func (his *His) GetSummaryWithId(patientId string, encounterId string) (sum Summary, err error) {
	summary, err := his.GetPatientDataSummary(patientId)
	if err != nil {
		return sum, err
	}
	idx := slices.IndexFunc(summary, func(s Summary) bool {
		return s.Id == encounterId
	})
	if idx == -1 {
		err = fmt.Errorf("encounter not found")
		return
	}
	return summary[idx], nil
}

func (his *His) GetPatientDataSummary(patientId string) (
	sum []Summary,
	err error,
) {
	patients := his.parsePatients()
	idx := slices.IndexFunc(patients, func(h HISPatient) bool {
		return h.CitizenId == patientId
	})
	if idx == -1 {
		err = fmt.Errorf("patient not found")
		return
	}
	// find patient file
	entries, err := os.ReadDir(path.Join(his.dir, "fhir"))
	if err != nil {
		log.Fatal(err)
	}
	id := patients[idx].PID
	// Filename FN_LN_id.json
	filenamepart := fmt.Sprint(id, ".json")
	fidx := slices.IndexFunc(entries, func(e fs.DirEntry) bool {
		return strings.Contains(e.Name(), filenamepart)
	})
	if fidx == -1 {
		err = fmt.Errorf("patient files not found")
		return
	}
	filename := entries[fidx].Name()
	sum, err = summarizeInfo(path.Join(his.dir, "fhir", filename))
	return
}

// Summarization

type Types struct {
	Name string `json:"text"`
}

func (t *Types) UnmarshalJSON(buf []byte) error {
	tmp1 := []struct {
		Text string `json:"text"`
	}{}
	if err := json.Unmarshal(buf, &tmp1); err == nil {
		t.Name = tmp1[0].Text
		return err
	}
	t.Name = ""
	return nil
}

type File struct {
	Entries []struct {
		FullUrl  string `json:"fullUrl"`
		Resource struct {
			ResourceType string `json:"resourceType"`
			Types        Types  `json:"type"`
			Code         struct {
				Text string `json:"text"`
			} `json:"code"`
			ValueQuantity struct {
				Unit  string  `json:"unit"`
				Value float32 `json:"value"`
			} `json:"valueQuantity"`
			Period struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"period"`
			ReasonCode []struct {
				Coding []struct {
					Display string `json:"display"`
				} `json:"coding"`
			} `json:"reasonCode"`
			Encounter struct {
				Reference string `json:"reference"`
			} `json:"encounter"`
		} `json:"resource"`
	} `json:"entry"`
}

type Summary struct {
	Encounter    `json:"Encounter"`
	Observations []Observation `json:"Observations"`
}

type Encounter struct {
	Id     string `json:"Id"`
	Reason string `json:"Reason"`
	Name   string `json:"Name"`
	Start  string `json:"Start"`
}
type Observation struct {
	Id        string  `json:"Id"`
	Encounter string  `json:"Encounter"`
	Name      string  `json:"Name"`
	Value     float32 `json:"Value"`
	Unit      string  `json:"Unit"`
}

func summarizeInfo(filePath string) (
	summary []Summary,
	err error,
) {
	fhirData := File{}

	f, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
		err = fmt.Errorf("could not read info")
		return
	}
	err = json.NewDecoder(f).Decode(&fhirData)
	if err != nil {
		fmt.Println(err)
		err = fmt.Errorf("could not read info")
		return
	}

	// Parse Data
	enc := map[string]Encounter{}
	obs := []Observation{}
	for _, e := range fhirData.Entries {
		if e.Resource.ResourceType == "Encounter" {
			reason := ""
			if len(e.Resource.ReasonCode) > 0 {
				reasonCode := e.Resource.ReasonCode[0]
				reason = reasonCode.Coding[0].Display
			}
			encounter := Encounter{
				Id:     e.FullUrl,
				Name:   e.Resource.Types.Name,
				Reason: reason,
				Start:  e.Resource.Period.Start,
			}
			enc[encounter.Id] = encounter
		} else if e.Resource.ResourceType == "Observation" {
			observation := Observation{
				Id:        e.FullUrl,
				Encounter: e.Resource.Encounter.Reference,
				Name:      e.Resource.Code.Text,
				Value:     e.Resource.ValueQuantity.Value,
				Unit:      e.Resource.ValueQuantity.Unit,
			}
			obs = append(obs, observation)
		}
	}
	// Merge data
	for eId, encounter := range enc {
		oblist := []Observation{}
		for _, ob := range obs {
			if ob.Encounter == eId {
				oblist = append(oblist, ob)
			}
		}
		sum := Summary{
			Encounter:    encounter,
			Observations: oblist,
		}
		summary = append(summary, sum)
	}
	return
}
