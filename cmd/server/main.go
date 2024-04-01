package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/containerd/containerd/log"
	"github.com/hashicorp/go-version"
	"github.com/naoki9911/fuse-diff-containerd/pkg/algorithm"
	"github.com/naoki9911/fuse-diff-containerd/pkg/image"
	"github.com/naoki9911/fuse-diff-containerd/pkg/update"
	"github.com/naoki9911/fuse-diff-containerd/pkg/utils"
	"github.com/sirupsen/logrus"
)

var logger = log.G(context.TODO())

var DiffDataGraphs map[string]*algorithm.DirectedGraph = map[string]*algorithm.DirectedGraph{}
var DiffDatas map[string]*update.DiffData = map[string]*update.DiffData{}
var DiffDatasLock = sync.Mutex{}
var tempDiffDir = "/tmp/d4c-server"

type diffServer struct {
	mergeConfig image.MergeConfig
}

func NewDiffServer(threadNum int) *diffServer {
	return &diffServer{
		mergeConfig: image.MergeConfig{
			ThreadNum: threadNum,
		},
	}
}

func getDiffTag(imageName, baseVersion, v string) string {
	return fmt.Sprintf("%s_%s-%s", imageName, baseVersion, v)
}

func (ds *diffServer) handleDeleteDiffData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		logger.Errorf("invalid method %s", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	DiffDatasLock.Lock()
	defer DiffDatasLock.Unlock()

	DiffDataGraphs = map[string]*algorithm.DirectedGraph{}
	DiffDatas = map[string]*update.DiffData{}
	logger.Info("cleaned DiffDatas")
}

func (ds *diffServer) handlePostDiffData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		logger.Errorf("invalid method %s", r.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	diffData, err := utils.UnmarshalJsonFromReader[update.DiffData](r.Body)
	if err != nil {
		logger.Errorf("invalid request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err = os.Stat(diffData.FileName)
	if err != nil {
		logger.Errorf("failed to stat dimg %s err=%v", diffData.FileName, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err = os.Stat(diffData.ConfigPath)
	if err != nil {
		logger.Errorf("failed to stat config %s err=%v", diffData.ConfigPath, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	DiffDatasLock.Lock()
	defer DiffDatasLock.Unlock()

	if _, ok := DiffDataGraphs[diffData.ImageName]; !ok {
		DiffDataGraphs[diffData.ImageName] = algorithm.NewDirectedGraph()
	}
	v, err := version.NewVersion(diffData.Version)
	if err != nil {
		logger.Errorf("invalid request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	baseVersion := "base"
	if diffData.BaseVersion != "" {
		baseVer, err := version.NewVersion(diffData.BaseVersion)
		if err != nil {
			logger.Errorf("invalid request err=%v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		baseVersion = baseVer.String()
	}
	DiffDataGraphs[diffData.ImageName].Add(baseVersion, v.String(), 1)
	DiffDatas[getDiffTag(diffData.ImageName, baseVersion, v.String())] = diffData
	logger.WithFields(logrus.Fields{"baseVersion": baseVersion, "version": v.String(), "Name": diffData.ImageName}).Infof("added diffData to dependency graph")

	logger.WithField("diffData", diffData).Info("added DiffDatas")
}

func (ds *diffServer) handleGetUpdateData(w http.ResponseWriter, r *http.Request) {
	req, err := utils.UnmarshalJsonFromReader[update.UpdateDataRequest](r.Body)
	if err != nil {
		logger.Errorf("invalid request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// select base image
	logger.WithFields(logrus.Fields{"requsetImage": req.RequestImage, "localImages": req.LocalImages}).Info("GetUpdateData")
	targetVersion, err := version.NewVersion(req.RequestImage.Version)
	if err != nil {
		logger.Errorf("invalid request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var selectedVersion *version.Version = nil
	localImages := make([]update.Image, 0)
	for i, img := range req.LocalImages {
		if img.Name != req.RequestImage.Name {
			continue
		}
		localImages = append(localImages, req.LocalImages[i])
		v, err := version.NewVersion(img.Version)
		if err != nil {
			logger.Errorf("invalid request err=%v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if targetVersion.LessThan(v) {
			continue
		}
		if selectedVersion == nil {
			selectedVersion = v
			continue
		}
		if v.LessThan(selectedVersion) {
			continue
		}
		selectedVersion = v
	}

	logger.WithFields(logrus.Fields{"targetVersion": targetVersion, "selectedVersion": selectedVersion, "localImages": localImages}).Info("base version is selected")

	DiffDatasLock.Lock()
	defer DiffDatasLock.Unlock()

	graph, ok := DiffDataGraphs[req.RequestImage.Name]
	if !ok {
		logger.Errorf("DiffDatas for image %s not found", req.RequestImage.Name)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Find diff data(selectedVersion -> targetVersion)
	baseVersion := "base"
	if selectedVersion != nil {
		baseVersion = selectedVersion.String()
	}

	logger.WithFields(logrus.Fields{"baseVersion": baseVersion, "version": targetVersion.String(), "Name": req.RequestImage.Name}).Infof("start to find best diffs")
	path, err := graph.ShortestPath(baseVersion, targetVersion.String())
	if err != nil {
		logger.Errorf("DiffDatas commbination for %s not found (base=%s target=%s)", req.RequestImage.Name, baseVersion, targetVersion.String())
		w.WriteHeader(http.StatusNotFound)
		return
	}

	pathStr := ""
	for i, p := range path {
		if i == 0 {
			pathStr = p.GetName()
		} else {
			pathStr += fmt.Sprintf(" -> %s", p.GetName())
		}
	}
	logger.WithField("Diffs", pathStr).Info("Diff Data to be transfered found")

	diffDataBytes := bytes.Buffer{}
	diffHeader := &image.DimgHeader{}
	configPath := ""

	if len(path) == 2 {
		tag := getDiffTag(req.RequestImage.Name, path[0].GetName(), path[1].GetName())
		diffData := DiffDatas[tag]
		diffF, err := os.Open(diffData.FileName)
		if err != nil {
			logger.Errorf("failed to load=%v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		diffHeader, _, err = image.LoadDimgHeader(diffF)
		if err != nil {
			logger.Errorf("failed to lead DimgHeader: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, err = diffF.Seek(0, 0)
		if err != nil {
			logger.Errorf("failed to seek dimg File: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, err = io.Copy(&diffDataBytes, diffF)
		if err != nil {
			logger.Errorf("failed to load=%v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		logger.Info("load done")
		configPath = diffData.ConfigPath
	} else {
		upperIdx := 2
		lowerTag := getDiffTag(req.RequestImage.Name, path[0].GetName(), path[1].GetName())
		lowerDiff := DiffDatas[lowerTag]
		lowerFileName := lowerDiff.FileName
		upperTag := getDiffTag(req.RequestImage.Name, path[1].GetName(), path[2].GetName())
		for upperIdx < len(path) {
			upperDiff := DiffDatas[upperTag]
			logger.WithFields(logrus.Fields{"lower": lowerFileName, "upper": upperDiff.FileName}).Info("merge")
			diffDataBytes = bytes.Buffer{}
			diffHeader, err = image.MergeDimg(lowerFileName, upperDiff.FileName, &diffDataBytes, ds.mergeConfig)
			if err != nil {
				logger.Errorf("failed to merge=%v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			logger.Info("merge done")
			if lowerFileName != lowerDiff.FileName {
				os.Remove(lowerFileName)
			}
			configPath = upperDiff.ConfigPath

			upperIdx += 1
			if upperIdx != len(path) {
				tempFile, err := os.CreateTemp(tempDiffDir, "diff-")
				if err != nil {
					logger.Error("failed to create temp file: %w", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_, err = tempFile.Write(diffDataBytes.Bytes())
				if err != nil {
					logger.Errorf("failed to write to tmp file: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				lowerFileName = tempFile.Name()
				logger.Infof("temp file saved at %s", lowerFileName)
				tempFile.Close()
				upperTag = getDiffTag(req.RequestImage.Name, path[upperIdx-1].GetName(), path[upperIdx].GetName())
			}
		}
	}

	res := update.UpdateDataResponse{
		Name:        req.RequestImage.Name,
		Version:     req.RequestImage.Version,
		BaseVersion: path[0].GetName(),
	}
	if res.BaseVersion == "base" {
		res.BaseVersion = ""
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
	configFile, err := os.Open(configPath)
	if err != nil {
		logger.Errorf("failed to open config file err=%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer configFile.Close()

	err = image.WriteCdimgHeader(configFile, diffHeader, int64(diffDataBytes.Len()), w)
	if err != nil {
		logger.Errorf("failed to cdimg header: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = io.Copy(w, &diffDataBytes)
	if err != nil {
		logger.Errorf("failed to write dimg: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logger.Infof("update sent")
}

func main() {
	threadNum := flag.Int("threadNum", 1, "Te number of threads to merge diffs")
	flag.Parse()
	os.RemoveAll(tempDiffDir)
	err := os.Mkdir(tempDiffDir, os.ModePerm)
	if err != nil {
		logger.Fatalf("failed to create tempDiffDir %s", tempDiffDir)
	}
	ds := NewDiffServer(*threadNum)
	http.HandleFunc("/update", ds.handleGetUpdateData)
	http.HandleFunc("/diffData/add", ds.handlePostDiffData)
	http.HandleFunc("/diffData/cleanup", ds.handleDeleteDiffData)
	logger.Info("started")
	err = http.ListenAndServe(":8081", nil)
	if err != nil {
		logger.Fatalf("failed to start server: %v", err)
	}
}
