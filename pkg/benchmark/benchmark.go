package benchmark

import (
	"fmt"
	"os"
	"time"
)

type Benchmark struct {
	logFile *os.File
}

type Metric struct {
	TaskName     string
	Timestamp    time.Time
	ElapsedMilli int
	Labels       []string
}

func NewBenchmark(path string) (*Benchmark, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, os.ModePerm)
	if err != nil {
		return nil, err
	}
	b := &Benchmark{
		logFile: file,
	}

	return b, nil
}

func (b *Benchmark) Close() error {
	err := b.logFile.Close()
	if err != nil {
		return err
	}

	return nil
}

func (b *Benchmark) AppendResult(m Metric) error {
	m.Timestamp = time.Now()
	col := fmt.Sprintf("%s,%s,%d", m.TaskName, m.Timestamp.Format(time.RFC3339), m.ElapsedMilli)
	for _, l := range m.Labels {
		col += fmt.Sprintf(",%s", l)
	}
	col += "\n"
	_, err := b.logFile.Write([]byte(col))
	if err != nil {
		return err
	}
	err = b.logFile.Sync()
	if err != nil {
		return err
	}
	return nil
}
