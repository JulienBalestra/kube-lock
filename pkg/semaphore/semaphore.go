package semaphore

import (
	"encoding/json"
	"time"
)

// Semaphore to store any holders. The name of each holder is the the key of the Holders map
type Semaphore struct {
	Max     int                `json:"max"`
	Holders map[string]*Holder `json:"holders"`
}

// Holder attributes
type Holder struct {
	Date   string `json:"date"`
	Reason string `json:"reason"`
}

// NewSemaphore instantiate a new semaphore
func NewSemaphore(maxHolders int) *Semaphore {
	return &Semaphore{
		Max:     maxHolders,
		Holders: map[string]*Holder{},
	}
}

// Unmarshal is a convenient wrapper over json
func (s *Semaphore) Unmarshal(b []byte) error {
	return json.Unmarshal(b, s)
}

// UnmarshalFromString is a convenient wrapper over json
func (s *Semaphore) UnmarshalFromString(str string) error {
	return s.Unmarshal([]byte(str))
}

// Marshal is a convenient wrapper over json
func (s *Semaphore) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

// MarshalToString is a convenient wrapper over json
func (s *Semaphore) MarshalToString() (string, error) {
	b, err := s.Marshal()
	return string(b), err
}

// SetHolder replace/add the given holder
func (s *Semaphore) SetHolder(name, reason string) {
	if s.Holders == nil {
		s.Holders = make(map[string]*Holder)
	}
	s.Holders[name] = &Holder{
		Date:   time.Now().Format("2006-01-02T15:04:05Z"),
		Reason: reason,
	}
}

// GetHolder returns the holder and if found
func (s *Semaphore) GetHolder(name string) (*Holder, bool) {
	if s.Holders == nil {
		return nil, false
	}
	h, ok := s.Holders[name]
	return h, ok
}
