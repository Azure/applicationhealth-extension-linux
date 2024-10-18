package seqno

import (
	"sync"

	"github.com/Azure/applicationhealth-extension-linux/internal/telemetry"
	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/azure-extension-platform/pkg/extensionerrors"
	"github.com/Azure/azure-extension-platform/pkg/seqno"
)

type SequenceNumberManager interface {
	// GetCurrentSequenceNumber returns the current sequence number the extension is using
	GetCurrentSequenceNumber(name, version string) (uint, error)

	// GetSequenceNumber retrieves the sequence number from the MRSEQ file
	GetSequenceNumber(name, version string) (uint, error)

	// SetSequenceNumber sets the sequence number to the MRSEQ file.
	SetSequenceNumber(name, version string, seqNo uint) error

	// FindSeqNum returns the requested the sequence number from either the environment variable or
	// the most recently used file under the config folder.
	// Note that this is different than just choosing the highest number, which may be incorrect
	FindSeqNum(configFolder string) (uint, error)
}

type SeqNumManager struct{}

var (
	instance SequenceNumberManager
	once     sync.Once
)

// GetInstance returns the singleton instance of SequenceNumberManager
func GetInstance() SequenceNumberManager {
	once.Do(func() {
		if instance == nil {
			instance = &SeqNumManager{}
		}
	})
	return instance
}

// SetInstance allows setting a custom instance for testing purposes
func SetInstance(customInstance SequenceNumberManager) {
	instance = customInstance
}

func (s *SeqNumManager) GetSequenceNumber(name string, version string) (uint, error) {
	retriever := &seqno.ProdSequenceNumberRetriever{}
	return retriever.GetSequenceNumber(name, version)
}

// SetSequenceNumber sets the sequence number for the given extension name and version.
// It takes the extension name, extension version, and sequence number as parameters.
// The sequence number is an integer that represents the order in which the extension was installed.
// It returns an error if there was a problem setting the sequence number.
func (s *SeqNumManager) SetSequenceNumber(name, version string, seqNo uint) error {
	return seqno.SetSequenceNumber(name, version, seqNo)
}

// FindSeqNum returns the requested the sequence number from either the environment variable or
// the most recently used file under the config folder.
// Note that this is different than just choosing the highest number, which may be incorrect
func (s *SeqNumManager) FindSeqNum(configFolder string) (uint, error) {
	return seqno.FindSeqNum(logging.NewNopLogger(), configFolder)
}

// GetCurrentSequenceNumber returns the current sequence number the extension is using
func (s *SeqNumManager) GetCurrentSequenceNumber(name, version string) (sn uint, _ error) {
	sequenceNumber, err := s.GetSequenceNumber(name, version)
	if err == extensionerrors.ErrNotFound || err == extensionerrors.ErrNoMrseqFile {
		// If we can't find the sequence number, then it's possible that the extension
		// hasn't been installed yet. Go back to 0.
		telemetry.SendEvent(telemetry.InfoEvent, telemetry.MainTask, "Couldn't find current sequence number, likely first execution of the extension, returning sequence number 0")
		return 0, nil
	}

	return sequenceNumber, err
}

// New returns the singleton instance of SequenceNumberManager
func New() SequenceNumberManager {
	return GetInstance()
}
