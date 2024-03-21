package pollinghandler

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"simplemts/lib"
	db "simplemts/lib/database"
	"slices"
	"sync"
	"time"
)

const DISABLE_EMAIL = false

type PollingHandler struct {
	client             lib.Requester
	serverURL          string
	tickerPaused       bool
	Database           *db.Database
	duration_s         int
	destPayloadDir     string
	resultDir          string
	originPayloadDir   string
	uploadDir          string
	staffGrantEmail    map[int]bool
	staffCompleteEmail map[int]bool
	docCompleteEmail   map[int]bool
	docNotGrantEmail   map[int]bool
}

func NewPollingHandler(duration_s int, client lib.Requester, database *db.Database, serverURL string) (ph PollingHandler) {
	handler := PollingHandler{
		duration_s:         duration_s,
		client:             client,
		tickerPaused:       true,
		Database:           database,
		serverURL:          serverURL,
		destPayloadDir:     lib.GetEnv("DEST_PAYLOAD_DIR", "../../client/download"),
		resultDir:          lib.GetEnv("DEST_RESULT_DIR", "../../client/download-result"),
		originPayloadDir:   lib.GetEnv("ORIGIN_PAYLOAD_DIR", "../../client-upload"),
		uploadDir:          lib.GetEnv("ORIGIN_UPLOAD_DIR", "../../client-upload"),
		staffGrantEmail:    map[int]bool{},
		staffCompleteEmail: map[int]bool{},
		docCompleteEmail:   map[int]bool{},
		docNotGrantEmail:   map[int]bool{},
	}
	handler.clearEmail()
	return handler
}

func (ph *PollingHandler) Run() {
	ticker := time.NewTicker(time.Second * time.Duration(ph.duration_s))
	ph.tickerPaused = false
	done := make(chan bool)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			ph.HandleTick()
		}
	}
}

type PollData = struct {
	Id             int               `json:"Id" validate:"required"`
	ReferralStatus db.ReferralStatus `json:"ReferralStatus" validate:"required"`
}

func (ph *PollingHandler) requestDecode(path string, targetCode int, response any) (err error) {
	resp, code, err := ph.client.MakeGetRequestRaw(ph.serverURL + path)
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("could make request: %d %s", code, resp)
	}
	err = lib.DecodeValidate(response, resp)
	if err != nil {
		return fmt.Errorf(err.Error())
	}
	return nil
}

func (ph *PollingHandler) HandleTick() {
	// if ph.tickerPaused {
	// 	return
	// }
	ph.tickerPaused = true // "Soft" Process locking

	response := struct {
		Referrals []PollData `json:"referrals" validate:"required"`
	}{}

	err := ph.requestDecode("/incoming", 200, &response)
	if err != nil {
		fmt.Println("Could not get incoming requests from server:", err)
		ph.tickerPaused = false
		return
	}
	// Use response
	for _, val := range response.Referrals {
		ph.HandleIncoming(val)
	}

	err = ph.requestDecode("/outgoing", 200, &response)
	if err != nil {
		fmt.Println("Could not get outgoing requests from server:", err)
		ph.tickerPaused = false
		return
	}
	// Use response
	for _, val := range response.Referrals {
		ph.HandleOutgoing(val)
	}

	ph.tickerPaused = false // TODO Or not
}

// func checksumBlock(inpath string, blockSize int64) (checksum string, err error) {
// 	hash := sha256.New()
// 	out, err := os.Open(inpath)
// 	if err != nil {
// 		return
// 	}
// 	defer out.Close()
// 	if _, err = io.CopyN(hash, out, blockSize); err != nil {
// 		return
// 	}
// 	checksum = hex.EncodeToString(hash.Sum(nil))
// 	return
// }

func checksumFile(inpath string) (checksum string, err error) {
	hash := sha256.New()
	out, err := os.Open(inpath)
	if err != nil {
		return
	}
	defer out.Close()
	if _, err = io.Copy(hash, out); err != nil {
		return
	}
	checksum = hex.EncodeToString(hash.Sum(nil))
	return
}

func fileEncrypt(filePath string, outpath string, secretKey []byte) (err error) {
	block, err := aes.NewCipher(secretKey)
	if err != nil {
		return
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}
	plainText, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return
	}
	cipherText := gcm.Seal(nonce, nonce, plainText, nil)
	f, err := lib.CreateFile(outpath)
	if err != nil {
		return
	}
	f.Write(cipherText)
	return
}

func encryptPayload(uploadDir string, payloadDir string, referralId int) (
	result struct {
		PayloadKey string          `json:"PayloadKey"`
		Files      []db.FileObject `json:"Files" validate:"required,unique=Name,dive"`
	},
	err error,
) {
	key := make([]byte, 32)
	_, err = rand.Read(key)
	if err != nil {
		return
	}
	result.PayloadKey = hex.EncodeToString(key) // TODO encrypt
	// get files
	referralUploadDir := path.Join(uploadDir, fmt.Sprint(referralId))
	referralPayloadDir := path.Join(payloadDir, fmt.Sprint(referralId))
	// files
	fileListing, err := os.ReadDir(path.Join(referralUploadDir, "files"))
	if err != nil {
		return
	}
	// TODO danger, listing ReferralData.json and [any] together, can error same name
	for _, file := range fileListing {
		err = fileEncrypt(path.Join(referralUploadDir, "files", file.Name()), path.Join(referralPayloadDir, file.Name()), key)
		if err != nil {
			return
		}
		var sum string
		sum, err = checksumFile(path.Join(referralPayloadDir, file.Name()))
		if err != nil {
			return
		}
		fileObject := db.FileObject{
			Name:     file.Name(),
			Checksum: sum,
		}
		result.Files = append(result.Files, fileObject)
	}
	return
}

// func makeChunks(uploadDir string, referralId int) {
// 	uDir := path.Join(uploadDir, fmt.Sprint(referralId))
// 	// files
// 	fileListing, err := os.ReadDir(path.Join(uDir, "encryption"))
// 	if err != nil {
// 		return
// 	}
// 	// TODO danger, listing ReferralData.json and [any] together, can error same name
// 	// for _, file := range fileListing {
// 	file := fileListing[0]
// 	f, err := os.Open(path.Join(uDir, "encryption", file.Name()))
// 	if err != nil {
// 		fmt.Fprintln(os.Stderr, err)
// 		return
// 	}
// 	defer f.Close()
// 	stride := 1 << 20 // block = 1MB 1 << 20
// 	r := bufio.NewReader(f)
// 	buf := make([]byte, 0, stride)
// 	for {
// 		n, err := io.ReadFull(r, buf[:cap(buf)])
// 		buf = buf[:n]
// 		if err != nil {
// 			if err == io.EOF {
// 				break
// 			}
// 			if err != io.ErrUnexpectedEOF {
// 				fmt.Fprintln(os.Stderr, err)
// 				break
// 			}
// 		}

// 		fmt.Println("read n bytes...", n)
// 		// process buf
// 	}
// 	// }
// }

func (ph *PollingHandler) HandleOutgoing(data PollData) {
	// Handle 1 outgoing
	referralId := data.Id
	referralPayloadDir := path.Join(ph.originPayloadDir, fmt.Sprintf("%d", referralId))
	switch data.ReferralStatus {
	case db.Granted:
		fmt.Println("Granted", referralId)
		_, err := os.Stat(referralPayloadDir)
		if err == nil {
			// no errors
			fmt.Println("already uploaded")
			break
		}
		if !os.IsNotExist(err) {
			// other error
			fmt.Println(err)
			break
		}
		// dir not found
		fetchfiles, err := encryptPayload(ph.uploadDir, ph.originPayloadDir, referralId)
		if err != nil {
			fmt.Println(err)
			break
		}
		fileString, err := json.Marshal(fetchfiles)
		if err != nil {
			fmt.Println(err)
			break
		}
		resp, code, err := ph.client.MakeJsonRequest(
			fmt.Sprintf(ph.serverURL+"/%d/upload", referralId),
			string(fileString))
		if err != nil {
			fmt.Println("file upload init error: ", err)
			break
		}
		if code != 201 {
			fmt.Println("Could not upload files ", code, resp)
			os.RemoveAll(referralPayloadDir)
		}
	case db.UploadIncomplete:
		fmt.Println("Begin Upload")
		// chunk begin
		request := struct {
			ChunkFiles []db.ChunkFile `json:"ChunkFiles"`
		}{}
		fileListing, err := os.ReadDir(referralPayloadDir)
		if err != nil {
			fmt.Println(err)
			break
		}
		for _, files := range fileListing {
			checksum, err := checksumFile(path.Join(referralPayloadDir, files.Name()))
			if err != nil {
				fmt.Println(err)
				break
			}
			chunk := db.ChunkFile{
				Name: files.Name(),
				Chunks: []db.Chunk{
					{
						Checksum: checksum,
						SizeKB:   1,
					},
				},
			}
			request.ChunkFiles = append(request.ChunkFiles, chunk)
		}
		chunkJson, err := json.Marshal(request)
		if err != nil {
			fmt.Println(err)
		}
		resp, code, err := ph.client.MakeJsonRequest(
			fmt.Sprintf(ph.serverURL+"/%d/upload/begin", referralId),
			string(chunkJson))
		if err != nil {
			fmt.Println("chunk begin error: ", err)
			break
		}
		if code != 201 {
			fmt.Println("Could not chunk begin files ", code, resp)
			break
		}
		for _, file := range fileListing {
			f, err := os.Open(path.Join(referralPayloadDir, file.Name()))
			if err != nil {
				fmt.Println("chunk begin error: ", err)
				break
			}
			defer f.Close()
			resp, code, err := ph.client.MakePostBinary(fmt.Sprintf(ph.serverURL+"/%d/upload/file/%s/0", referralId, file.Name()), f)
			if err != nil {
				fmt.Println("chunk upload error: ", err)
				break
			}
			if code != 200 {
				fmt.Println("Chunk upload error: ", code, resp)
				break
			}
		}
		resp, code, err = ph.client.MakeJsonRequest(fmt.Sprintf(ph.serverURL+"/%d/upload/complete", referralId), "")
		if err != nil {
			fmt.Println("chunk completion error: ", err)
			break
		}
		if code != 200 {
			fmt.Println("Chunk completion error: ", code, resp)
			break
		}
		fmt.Println("Chunk upload complete")
	case db.Complete:
		if DISABLE_EMAIL {
			return
		}
		if _, has := ph.docCompleteEmail[referralId]; has {
			return
		}
		_, dest, patientName, date := ph.getEmailInfo(false, referralId)
		emailDocComplete("test", patientName, referralId, date, dest)
		ph.docCompleteEmail[referralId] = true
	case db.NotGranted:
		if DISABLE_EMAIL {
			return
		}
		if _, has := ph.docNotGrantEmail[referralId]; has {
			return
		}
		_, dest, patientName, date := ph.getEmailInfo(false, referralId)
		emailDocNotGrant("test", patientName, referralId, date, dest)
		ph.docNotGrantEmail[referralId] = true
	}
}

func fileDecrypt(inpath string, outPath string, secretKey []byte) (err error) {
	cipherText, err := os.ReadFile(inpath)
	if err != nil {
		return
	}
	block, err := aes.NewCipher([]byte(secretKey))
	if err != nil {
		return
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := cipherText[:nonceSize], cipherText[nonceSize:]
	plaintext, err := gcm.Open(nil, []byte(nonce), []byte(ciphertext), nil)
	if err != nil {
		return
	}
	f, err := lib.CreateFile(outPath)
	if err != nil {
		return
	}
	f.Write(plaintext)
	if err != nil {
		return
	}
	return
}

func (ph *PollingHandler) getEmailInfo(isIncoming bool, referralId int) (origin string, dest string, fullName string, date string) {
	// referral
	type referral struct {
		Id             int               `json:"Id" validate:"required"`
		ReferralStatus db.ReferralStatus `json:"ReferralStatus" validate:"required"`
		db.PatientObject
		Origin      string `json:"Origin" validate:"required"`
		Destination string `json:"Destination" validate:"required"`
		Reason      string `json:"Reason" validate:"required"`
		Created     int64  `json:"Created" validate:"required"`
	}
	response := struct {
		Referrals []referral `json:"referrals" validate:"required,dive"`
	}{}
	url := "/incoming"
	if !isIncoming {
		url = "/outgoing"
	}
	err := ph.requestDecode(url, 200, &response)
	if err != nil {
		fmt.Println("Could not get referrals", err)
		return
	}
	// hospital
	// type hospitalSt struct {
	// 	HospitalId   string `json:"HospitalId" validate:"required"`
	// 	HospitalName string `json:"HospitalName" validate:"required"`
	// }
	hospitals := []db.Hospital{}
	resp, code, err := ph.client.MakeGetRequestRaw(ph.serverURL + "/hospitals")
	if err != nil {
		fmt.Println("get hospital error, ", err)
	}
	if code != 200 {
		fmt.Printf("could make request: %d %s\n", code, resp)
		return
	}
	err = json.NewDecoder(resp).Decode(&hospitals)
	if err != nil {
		fmt.Printf("could not decode request: %d %s\n", code, resp)
		return
	}
	idx := slices.IndexFunc(response.Referrals, func(r referral) bool {
		return r.Id == referralId
	})
	targetReferral := response.Referrals[idx]
	didx := slices.IndexFunc(hospitals, func(r db.Hospital) bool {
		return r.HospitalId == targetReferral.Destination
	})
	dest = hospitals[didx].HospitalName
	oidx := slices.IndexFunc(hospitals, func(r db.Hospital) bool {
		return r.HospitalId == targetReferral.Origin
	})
	origin = hospitals[oidx].HospitalName
	// fmt.Println(targetReferral.FirstName, targetReferral.Id, targetReferral.Created, targetHospital.HospitalName)
	fullName = fmt.Sprintf("%s %s %s", targetReferral.Prefix, targetReferral.FirstName, targetReferral.LastName)
	date = time.Unix(targetReferral.Created, 0).Format("2006-01-02")
	return
}

func (ph *PollingHandler) HandleIncoming(data PollData) {
	// Handle 1 incoming
	referralId := data.Id
	switch data.ReferralStatus {
	case db.Consented:
		// Grant
		if DISABLE_EMAIL {
			return
		}
		if _, has := ph.staffGrantEmail[referralId]; has {
			return
		}
		origin, dest, _, date := ph.getEmailInfo(true, referralId)
		emailStaffGrant(referralId, date, dest, origin)
		ph.staffGrantEmail[referralId] = true
	case db.Complete:
		if DISABLE_EMAIL {
			return
		}
		// Complete
		if _, has := ph.staffCompleteEmail[referralId]; has {
			return
		}
		origin, dest, _, date := ph.getEmailInfo(true, referralId)
		emailStaffComplete(referralId, date, dest, origin)
		ph.staffCompleteEmail[referralId] = true
	case db.UploadComplete:
		// Get files list
		fileList, payloadKey, err := ph.DownloadList(referralId)
		if err != nil {
			fmt.Printf("Could not begin download: %s", err)
			return
		}
		// Check complete
		downloadDir := path.Join(ph.destPayloadDir, fmt.Sprintf("referral-%d", referralId))
		_, err = os.Stat(downloadDir)
		if err != nil {
			// open dir errors
			if !os.IsNotExist(err) {
				// other error
				fmt.Println("download error: ", err)
				break
				// Dir not exist error, continue
			}
		} else {
			// Dir exists
			dirsList, err := os.ReadDir(downloadDir)
			if err != nil {
				fmt.Printf("Could not read download dir: %s", err)
				return
			}
			for _, fileItem := range fileList {
				idx := slices.IndexFunc(dirsList, func(de fs.DirEntry) bool {
					return de.Name() == fileItem.Name
				})
				if idx < 0 {
					break // files not included
				} else {
					// Complete
					fmt.Println("Download Complete for", referralId)
					return
				}
			}
			// Dir incomplete
		}
		// Download
		var wg sync.WaitGroup

		downloadpath := path.Join(
			ph.destPayloadDir,
			fmt.Sprintf("referral-%d", referralId),
		)
		for _, file := range fileList {
			if file.UploadStatus == db.CompleteUpload {
				// download complete files
				wg.Add(1)
				go func(filename string) {
					defer wg.Done()
					err := ph.DownloadFile(downloadpath, referralId, filename)
					// fmt.Println(err)
					if err != nil {
						fmt.Println("Download error: ", err)
					}
				}(file.Name)
			}
		}
		wg.Wait()
		decryptDir := path.Join(ph.resultDir, fmt.Sprintf("referral-%d", referralId))
		decryptKey, err := hex.DecodeString(payloadKey)
		if err != nil {
			fmt.Println(err)
			break
		}
		for _, file := range fileList {
			err := fileDecrypt(path.Join(downloadpath, file.Name), path.Join(decryptDir, file.Name), decryptKey)
			if err != nil {
				fmt.Println("decryption error", err)
				break
			}
		}
		ph.client.MakeJsonRequest(ph.serverURL+fmt.Sprintf("/%d/complete", referralId), "")
	}
}
