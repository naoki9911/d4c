package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Benchmark struct {
	logFile       *os.File
	writeLock     sync.Mutex
	defaultLabels map[string]string
}

type Metric struct {
	TaskName     string            `json:"taskName"`
	Timestamp    time.Time         `json:"timestamp"`
	ElapsedMilli int               `json:"elapsedMilliseconds"`
	Size         int64             `json:"size"`
	Labels       map[string]string `json:"labels"`
}

func (m *Metric) AddLabels(labels map[string]string) {
	for k, v := range labels {
		m.Labels[k] = v
	}
}

func NewBenchmark(path string) (*Benchmark, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, os.ModePerm)
	if err != nil {
		return nil, err
	}
	b := &Benchmark{
		logFile:   file,
		writeLock: sync.Mutex{},
	}

	return b, nil
}

func (b *Benchmark) SetDefaultLabels(l map[string]string) {
	b.defaultLabels = l
}

func (b *Benchmark) Close() error {
	err := b.logFile.Close()
	if err != nil {
		return err
	}

	return nil
}

func (b *Benchmark) AppendResult(m Metric) error {
	if b.defaultLabels != nil {
		m.AddLabels(b.defaultLabels)
	}
	m.Timestamp = time.Now()
	bytes, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal Metric: %v", err)
	}
	b.writeLock.Lock()
	defer b.writeLock.Unlock()
	_, err = b.logFile.Write(append(bytes, []byte("\n")...))
	if err != nil {
		return err
	}
	return nil
}
