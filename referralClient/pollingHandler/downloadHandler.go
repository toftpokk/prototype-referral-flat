package pollinghandler

import (
	"fmt"
	"io"
	"path"
	"simplemts/lib"
	db "simplemts/lib/database"
)

type ChunkFile = db.ChunkFile

type Chunk = db.Chunk

const (
	Incomplete = db.IncompleteChunk
	Complete   = db.CompleteChunk
)

type FileTracking = struct {
	UploadStatus db.UploadStatus `json:"UploadStatus"`
	Name         string          `json:"Name"`
	Checksum     string          `json:"Checksum"`
}

func (ph *PollingHandler) DownloadList(referralId int) ([]FileTracking, string, error) {
	// Check file list
	response := struct {
		PayloadKey string         `json:"PayloadKey"`
		Files      []FileTracking `json:"Files"`
	}{}
	err := ph.requestDecode(fmt.Sprintf("/%d/download", referralId), 200, &response)
	if err != nil {
		return []FileTracking{}, "", err
	}
	// Create tracking files
	parentPath := path.Join(ph.destPayloadDir, fmt.Sprint(referralId)) // file exists in /download/referralId/fileId
	for _, file := range response.Files {
		f := db.File{
			FileObject: db.FileObject{
				Name:     file.Name,
				Checksum: file.Checksum,
			},
			Referral:   referralId,
			ParentPath: parentPath,
		}
		// Skip incomplete
		if file.UploadStatus == db.IncompleteUpload {
			continue
		}
		_, ok := ph.Database.ClientCreateFile(f)
		if !ok {
			return response.Files, "", fmt.Errorf("could not create file for referral '%d'", referralId)
		}
	}
	return response.Files, response.PayloadKey, nil
}

func (ph *PollingHandler) DownloadFile(downloadPath string, referralId int, filename string) error {
	filereader, statusCode, err := ph.client.MakeGetRequestRaw(ph.serverURL + fmt.Sprintf("/%d/download/%s", referralId, filename))
	if err != nil {
		return err
	}
	if statusCode != 200 {
		return fmt.Errorf("request failed %s", err)
	}
	f, err := lib.CreateFile(path.Join(downloadPath, filename))
	if err != nil {
		return err
	}
	_, err = io.Copy(f, filereader)
	if err != nil {
		return err
	}
	return nil
}
