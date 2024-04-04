package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/containerd/containerd/log"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	"github.com/opencontainers/go-digest"
)

var logger = log.G(context.TODO())

const imageStorePath = "/tmp/d4c-server/images"

type diffImage struct {
	dimgId digest.Digest
}

type DiffServer struct {
	mergeConfig image.MergeConfig
	serverMux   *http.ServeMux
	lock        sync.Mutex

	dimgStore *image.DimgStore
	imageTags map[string]diffImage
}

func NewDiffServer(mc image.MergeConfig) (*DiffServer, error) {
	server := &DiffServer{
		mergeConfig: mc,
		serverMux:   http.NewServeMux(),
		lock:        sync.Mutex{},
	}

	err := server.clearAll()
	if err != nil {
		return nil, err
	}

	server.serverMux.HandleFunc("/update", server.handleGetUpdateData)
	server.serverMux.HandleFunc("/diffData/add", server.handlePostDiffData)
	server.serverMux.HandleFunc("/cleanup", server.handleDeleteAll)

	return server, nil
}

func (ds *DiffServer) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, ds.serverMux)
}

func (ds *DiffServer) clearAll() error {
	err := os.RemoveAll(imageStorePath)
	if err != nil {
		return fmt.Errorf("failed to remove image store %v: %v", imageStorePath, err)
	}

	dimgStore, err := image.NewDimgStore(imageStorePath)
	if err != nil {
		return fmt.Errorf("failed to create DimgStore at %s: %v", imageStorePath, err)
	}

	ds.dimgStore = dimgStore
	ds.imageTags = map[string]diffImage{}

	return nil
}

func (ds *DiffServer) handleDeleteAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		logger.Errorf("invalid method %s", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ds.lock.Lock()
	defer ds.lock.Unlock()
	err := ds.clearAll()
	if err != nil {
		logger.Errorf("failed to clear all: %v", err)
		return
	}
	logger.Info("cleaned DiffDatas")
	w.WriteHeader(http.StatusOK)
}

func (ds *DiffServer) handlePostDiffData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		logger.Errorf("invalid method %s", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	diffData, err := utils.UnmarshalJsonFromReader[DiffData](r.Body)
	if err != nil {
		logger.Errorf("invalid request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cdimgFile, err := image.OpenCdimgFile(diffData.CdimgPath)
	if err != nil {
		logger.Errorf("failed to open cdimg %s: %v", diffData.CdimgPath, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer cdimgFile.Close()

	dimgPath := filepath.Join(imageStorePath, utils.GetRandomId("temp")+".dimg")
	dimgFile, err := os.Create(dimgPath)
	if err != nil {
		logger.Errorf("failed to create temporarly dimg file at %s: %v", dimgPath, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer dimgFile.Close()

	err = cdimgFile.WriteDimg(dimgFile)
	if err != nil {
		logger.Errorf("failed to write dimg file at %s: %v", dimgPath, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = ds.dimgStore.AddDimg(dimgPath, cdimgFile.Header.ConfigBytes)
	if err != nil {
		logger.Errorf("failed to add dimg %s: %v", dimgPath, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Infof("successfully added DiffData(Name=%s Version=%s CdimgPath=%s)", diffData.ImageTag.Name, diffData.ImageTag.Version, diffData.CdimgPath)

	if !diffData.ImageTag.Exist() {
		w.WriteHeader(http.StatusOK)
		return
	}

	ds.lock.Lock()
	ds.imageTags[diffData.ImageTag.String()] = diffImage{
		dimgId: cdimgFile.Dimg.Header().Id,
	}
	logger.Infof("successfully registered ImageTag %s (Id=%s)", diffData.ImageTag.String(), cdimgFile.Dimg.Header().Id)
	ds.lock.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (ds *DiffServer) handleGetUpdateData(w http.ResponseWriter, r *http.Request) {
	req, err := utils.UnmarshalJsonFromReader[UpdateDataRequest](r.Body)
	if err != nil {
		logger.Errorf("invalid request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ds.lock.Lock()
	defer ds.lock.Unlock()

	img, ok := ds.imageTags[req.RequestImage.String()]
	if !ok {
		logger.Errorf("not found digest for %s", req.RequestImage.String())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logger.Infof("client's local dimgs are %v", req.LocalDimgs)
	logger.Infof("DimgId for requested image %s is %v", req.RequestImage.String(), img)

	req.LocalDimgs = append(req.LocalDimgs, "")
	selectedDimgPaths, err := ds.dimgStore.GetDimgEntriesWithDimgIds(img.dimgId, req.LocalDimgs)
	if err != nil {
		logger.Errorf("failed to get dimgs from %v to %v", img.dimgId, req.LocalDimgs)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	selectedDimgDigests := []digest.Digest{}
	dimgsMsg := ""
	for _, dimg := range selectedDimgPaths {
		dimgsMsg += fmt.Sprintf("%s -> %s, ", dimg.Id, dimg.ParentId)
		selectedDimgDigests = append(selectedDimgDigests, dimg.Digest())
	}
	logger.Infof("Dimgs are sent to client %s", dimgsMsg)

	lowerDimg := selectedDimgPaths[len(selectedDimgPaths)-1]
	for idx := len(selectedDimgPaths) - 2; idx >= 0; idx-- {
		upperDimg := selectedDimgPaths[idx]
		mergedDimgPath := filepath.Join(imageStorePath, utils.GetRandomId("d4c-server")+".dimg")
		mergedFile, err := os.Create(mergedDimgPath)
		if err != nil {
			logger.Errorf("failed to create temporarly dimg %s: %v", mergedDimgPath, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer mergedFile.Close()

		logger.Infof("merge %s and %s", lowerDimg.Digest(), upperDimg.Digest())
		header, err := image.MergeDimg(lowerDimg.Path, upperDimg.Path, mergedFile, ds.mergeConfig)
		if err != nil {
			logger.Errorf("failed to merge dimgs: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		lowerDimg.DimgHeader = *header
		lowerDimg.Path = mergedDimgPath
	}
	resDimg := lowerDimg

	resDimgFile, err := image.OpenDimgFile(resDimg.Path)
	if err != nil {
		logger.Errorf("failed to open dimg %s: %v", resDimg.Path, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resDimgFile.Close()

	resDimgFileStat, err := os.Stat(resDimg.Path)
	if err != nil {
		logger.Errorf("failed to stats dimg %s: %v", resDimg.Path, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	res := UpdateDataResponse{
		ImageTag: ImageTag{
			Name:    req.RequestImage.Name,
			Version: req.RequestImage.Version,
		},
		SourceDimgs: selectedDimgDigests,
	}
	resBytes, err := json.Marshal(res)
	if err != nil {
		logger.Errorf("failed to marshal json err=%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Update-Response-Length", strconv.Itoa(len(resBytes)))
	_, err = w.Write(resBytes)
	if err != nil {
		logger.Errorf("failed to send response json err=%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = image.WriteCdimgHeader(bytes.NewBuffer(resDimg.ConfigBytes), &resDimg.DimgHeader, resDimgFileStat.Size(), w)
	if err != nil {
		logger.Errorf("failed to cdimg header: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = resDimgFile.WriteAll(w)
	if err != nil {
		logger.Errorf("failed to write dimg: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logger.Infof("update sent")
}
