package bench

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aristanetworks/goarista/monotime"
)

type Progress struct {
	Processed uint64        `json:"processed"` // total bytes read or written so far
	Delta     uint64        `json:"delta"`     // bytes written since last event
	Duration  time.Duration `json:"duration"`  // time in ns since last event
}

// BPS returns the 'write/read speed' in bytes/s.
func (ev Progress) BPS() float64 {
	return (float64(ev.Delta) / float64(ev.Duration)) * float64(time.Second)
}

func mononow() time.Duration {
	return time.Duration(monotime.Now())
}

// ReadProgress reads JSON progress events in a file.
func ReadProgress(file string) ([]Progress, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	var pp []Progress
	dec := json.NewDecoder(fd)
	for {
		var p Progress
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			return pp, err
		}
		pp = append(pp, p)
	}
	return pp, nil
}

type Report struct {
	Name   string
	Events []Progress
}

// MustReadReports reads all given progress event files.
func MustReadReports(files []string) []Report {
	var reports []Report
	for _, file := range files {
		p, err := ReadProgress(file)
		if err != nil {
			log.Fatalf("%s: %v", file, err)
		}
		reports = append(reports, Report{
			Events: p,
			Name:   strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)),
		})
	}
	return reports
}

// copyBytes returns an exact copy of the provided bytes.
func copyBytes(b []byte) (copiedBytes []byte) {
	if b == nil {
		return nil
	}
	copiedBytes = make([]byte, len(b))
	copy(copiedBytes, b)
	return
}

// copySlices returns an exact copy of the provided slices.
func copySlices(s [][]byte) (copiedSlice [][]byte) {
	copiedSlice = make([][]byte, len(s))
	for i := 0; i < len(s); i++ {
		copy(copiedSlice[i], s[i])
	}
	return
}
