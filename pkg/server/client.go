package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type DiffClient struct {
	serverHost string
}

func NewDiffClient(serverHost string) *DiffClient {
	dc := &DiffClient{
		serverHost: serverHost,
	}

	return dc
}

func (dc *DiffClient) PushImage(cdimgPath string, imageTag *ImageTag) error {
	reqJson := DiffData{
		CdimgPath: cdimgPath,
	}
	if imageTag != nil {
		reqJson.ImageTag = *imageTag
	}

	jsonBytes, err := json.Marshal(reqJson)
	if err != nil {
		return fmt.Errorf("failed to marshal DiffData: %v", err)
	}

	res, err := http.Post(fmt.Sprintf("http://%s/diffData/add", dc.serverHost), "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to post DiffData: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", res.StatusCode)
	}
	return nil
}
