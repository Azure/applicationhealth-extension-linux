package seqno

import (
	"github.com/go-kit/log"

	"github.com/Azure/applicationhealth-extension-linux/pkg/logging"
	"github.com/Azure/azure-extension-platform/pkg/extensionerrors"
	"github.com/Azure/azure-extension-platform/pkg/seqno"
)

type SequenceNumberManager interface {
	// GetCurrentSequenceNumber returns the current sequence number the extension is using
	GetCurrentSequenceNumber(name, version string) (int, error)

	// GetSequenceNumber retrieves the sequence number from the MRSEQ file
	GetSequenceNumber(name, version string) (int, error)

	// SetSequenceNumber sets the sequence number to the MRSEQ file.
	SetSequenceNumber(name, version string, seqNo int) error

	// FindSeqNum returns the requested the sequence number from either the environment variable or
	// the most recently used file under the config folder.
	// Note that this is different than just choosing the highest number, which may be incorrect
	FindSeqNum(configFolder string) (int, error)
}

type SeqNumManager struct {
	GetCurrentSequenceNumberFunc func(lg log.Logger, name, version string) (int, error)
	GetSequenceNumberFunc        func(name, version string) (int, error)
	SequenceNumberSetterFunc     func(name, version string, seqNo int) error
	FindSeqNumFunc               func(configFolder string) (int, error)
}

func (s *SeqNumManager) GetCurrentSequenceNumber(name, version string) (int, error) {
	lg := logging.NewNopLogger()
	return s.GetCurrentSequenceNumberFunc(lg, name, version)
}

func (s *SeqNumManager) GetSequenceNumber(name string, version string) (int, error) {
	return s.GetSequenceNumberFunc(name, version)
}

func (s *SeqNumManager) SetSequenceNumber(name, version string, seqNo int) error {
	return s.SequenceNumberSetterFunc(name, version, seqNo)
}

func (s *SeqNumManager) FindSeqNum(configFolder string) (int, error) {
	return s.FindSeqNumFunc(configFolder)
}

func New() SequenceNumberManager {
	return &SeqNumManager{
		GetCurrentSequenceNumberFunc: GetCurrentSequenceNumberFunc(GetSequenceNumberFunc),
		GetSequenceNumberFunc:        GetSequenceNumberFunc,
		SequenceNumberSetterFunc:     SetSequenceNumber,
		FindSeqNumFunc:               FindSeqNum,
	}
}

func GetSequenceNumberFunc(name, version string) (int, error) {
	retriever := &seqno.ProdSequenceNumberRetriever{}
	seqNum, err := retriever.GetSequenceNumber(name, version)
	return int(seqNum), err
}

// SetSequenceNumber sets the sequence number for the given extension name and version.
// It takes the extension name, extension version, and sequence number as parameters.
// The sequence number is an integer that represents the order in which the extension was installed.
// It returns an error if there was a problem setting the sequence number.
func SetSequenceNumber(extName, extVersion string, seqNo int) error {
	return seqno.SetSequenceNumber(extName, extVersion, uint(seqNo))
}

// FindSeqNum finds the sequence number for the given config folder.
// It returns the sequence number as an integer and any error encountered.
func FindSeqNum(configFolder string) (int, error) {
	seqNum, err := seqno.FindSeqNum(logging.NewNopLogger(), configFolder)
	if err != nil {
		return 0, err
	}
	return int(seqNum), nil
}

func GetCurrentSequenceNumberFunc(getSequenceNumberFunc func(name, version string) (int, error)) func(lg log.Logger, name, version string) (int, error) {
	return func(lg log.Logger, name, version string) (int, error) {
		return getCurrentSequenceNumber(lg, getSequenceNumberFunc, name, version)
	}
}

// GetCurrentSequenceNumber returns the current sequence number the extension is using
func getCurrentSequenceNumber(lg log.Logger, getSequenceNumberFunc func(name, version string) (int, error), name, version string) (int, error) {
	sequenceNumber, err := getSequenceNumberFunc(name, version)
	if err == extensionerrors.ErrNotFound || err == extensionerrors.ErrNoMrseqFile {
		// If we can't find the sequence number, then it's possible that the extension
		// hasn't been installed yet. Go back to 0.
		lg.Log("event", "Couldn't find current sequence number, likely first execution of the extension, returning sequence number 0")
		return 0, nil
	}

	return int(sequenceNumber), err
}
