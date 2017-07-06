package core

import (
	"bytes"
	"encoding/gob"
	json0 "encoding/json"
	"errors"
	"fmt"
)

// UnmarshalJSON handles the deserializing of a message.
//
// We can't get away with off-the-shelf JSON, because
// we're using an interface type for MsgData, which causes problems
// on the decode side.
func (m *Msg) UnmarshalJSON(b []byte) error {
	// Use builtin json to unmarshall into aux
	var aux msgAux
	json0.Unmarshal(b, &aux)

	// The Op field in aux is already what we want for m.Op
	m.Op = aux.Op

	// decode the gob in aux.Data and put it in m.Data
	dec := gob.NewDecoder(bytes.NewBuffer(aux.Data))
	switch aux.Op {
	case Handshake:
		var h HandshakeMsg
		err := dec.Decode(&h)
		if err != nil {
			return errors.New("failed to decode HandshakeMsg")
		}
		m.Data = h
	case Have:
		var h HaveMsg
		err := dec.Decode(&h)
		if err != nil {
			return errors.New("failed to decode HaveMsg")
		}
		m.Data = h
	case Request:
		var r RequestMsg
		err := dec.Decode(&r)
		if err != nil {
			return errors.New("failed to decode RequestMsg")
		}
		m.Data = r
	case Data:
		var r DataMsg
		err := dec.Decode(&r)
		if err != nil {
			return errors.New("failed to decode DataMsg")
		}
		m.Data = r
	default:
		return errors.New("failed to decode message data")
	}

	return nil
}

// MarshalJSON handles the serializing of a message.
//
// See note above UnmarshalJSON for the reason for the custom MarshalJSON
func (m *Msg) MarshalJSON() ([]byte, error) {
	// Encode m.Data into a gob
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	switch m.Data.(type) {
	case HandshakeMsg:
		gob.Register(HandshakeMsg{})
		err := enc.Encode(m.Data.(HandshakeMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal HandshakeMsg: %v", err)
		}
	case HaveMsg:
		gob.Register(HaveMsg{})
		err := enc.Encode(m.Data.(HaveMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal HaveMsg: %v", err)
		}
	case RequestMsg:
		gob.Register(RequestMsg{})
		err := enc.Encode(m.Data.(RequestMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal RequestMsg: %v", err)
		}
	case DataMsg:
		gob.Register(DataMsg{})
		err := enc.Encode(m.Data.(DataMsg))
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal DataMsg: %v", err)
		}
	default:
		return nil, errors.New("failed to marshal message data")
	}

	// build an aux and marshal using built-in json
	aux := msgAux{Op: m.Op, Data: b.Bytes()}
	return json0.Marshal(aux)
}
