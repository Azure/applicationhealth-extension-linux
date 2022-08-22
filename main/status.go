package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

type StatusReport []StatusItem

type StatusItem struct {
	Version      float64 `json:"version"`
	TimestampUTC string  `json:"timestampUTC"`
	Status       Status  `json:"status"`
}

type StatusType string

const (
	StatusTransitioning StatusType = "transitioning"
	StatusError         StatusType = "error"
	StatusSuccess       StatusType = "success"
)

type Status struct {
	Operation                   string           `json:"operation"`
	ConfigurationAppliedTimeUTC string           `json:"configurationAppliedTime"`
	Status                      StatusType       `json:"status"`
	FormattedMessage            FormattedMessage `json:"formattedMessage"`
	SubstatusList               []SubstatusItem  `json:"substatus,omitempty"`
}

type FormattedMessage struct {
	Lang    string `json:"lang"`
	Message string `json:"message"`
}

type SubstatusItem struct {
	Name             string           `json:"name"`
	Status           StatusType       `json:"status"`
	FormattedMessage FormattedMessage `json:"formattedMessage"`
}

func NewStatus(t StatusType, operation, message string) StatusReport {
	now := time.Now().UTC().Format(time.RFC3339)
	return []StatusItem{
		{
			Version:      1.0,
			TimestampUTC: now,
			Status: Status{
				Operation:                   operation,
				ConfigurationAppliedTimeUTC: now,
				Status: t,
				FormattedMessage: FormattedMessage{
					Lang:    "en",
					Message: message,
				},
			},
		},
	}
}

func NewSubstatus(name string, t StatusType, message string) SubstatusItem {
	return SubstatusItem {
		Name:   name,
		Status: t,
		FormattedMessage: FormattedMessage{
			Lang:    "en",
			Message: message,
		},
	}
}

func (r StatusReport) AddSubstatus(t StatusType, name, message string, state HealthStatus) {
	if len(r) > 0 {
		substatusItem := SubstatusItem{
			Name:   name,
			Status: t,
			FormattedMessage: FormattedMessage{
				Lang:    "en",
				Message: message,
			},
		}
		r[0].Status.SubstatusList = append(r[0].Status.SubstatusList, substatusItem)
	}
}

func (r StatusReport) AddSubstatusItem(substatus SubstatusItem) {
	if len(r) > 0 {
		r[0].Status.SubstatusList = append(r[0].Status.SubstatusList, substatus)
	}
}

func (r StatusReport) marshal() ([]byte, error) {
	return json.MarshalIndent(r, "", "\t")
}

// Save persists the status message to the specified status folder using the
// sequence number. The operation consists of writing to a temporary file in the
// same folder and moving it to the final destination for atomicity.
func (r StatusReport) Save(statusFolder string, seqNum int) error {
	fn := fmt.Sprintf("%d.status", seqNum)
	path := filepath.Join(statusFolder, fn)
	tmpFile, err := ioutil.TempFile(statusFolder, fn)
	if err != nil {
		return fmt.Errorf("status: failed to create temporary file: %v", err)
	}
	tmpFile.Close()

	b, err := r.marshal()
	if err != nil {
		return fmt.Errorf("status: failed to marshal into json: %v", err)
	}
	if err := ioutil.WriteFile(tmpFile.Name(), b, 0644); err != nil {
		return fmt.Errorf("status: failed to write to path=%s error=%v", tmpFile.Name(), err)
	}

	if err := os.Rename(tmpFile.Name(), path); err != nil {
		return fmt.Errorf("status: failed to move to path=%s error=%v", path, err)
	}
	return nil
}
