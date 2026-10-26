package acp

import "encoding/json"

// Compaction patches distinguish omission (leave unchanged), null (clear), and
// a value (replace). The generated fields retain their existing types; presence
// flags record null on decode and allow callers to request it on encode.
// A non-nil slice/map is a value even when empty, regardless of its flag.
func (v CompactionUpdate) MarshalJSON() ([]byte, error) {
	type wire CompactionUpdate
	return marshalCompactionPatch(wire(v), v.Summary, v.Error, v.Meta, v.SummaryPresent, v.ErrorPresent, v.MetaPresent)
}

func (v *CompactionUpdate) UnmarshalJSON(data []byte) error {
	type wire CompactionUpdate
	var decoded wire
	fields, err := unmarshalCompactionPatch(data, &decoded)
	if err != nil {
		return err
	}
	_, decoded.SummaryPresent = fields["summary"]
	_, decoded.ErrorPresent = fields["error"]
	_, decoded.MetaPresent = fields["_meta"]
	*v = CompactionUpdate(decoded)
	return nil
}

func (v SessionCompactionUpdate) MarshalJSON() ([]byte, error) {
	type wire SessionCompactionUpdate
	return marshalCompactionPatch(wire(v), v.Summary, v.Error, v.Meta, v.SummaryPresent, v.ErrorPresent, v.MetaPresent)
}

func (v *SessionCompactionUpdate) UnmarshalJSON(data []byte) error {
	type wire SessionCompactionUpdate
	var decoded wire
	fields, err := unmarshalCompactionPatch(data, &decoded)
	if err != nil {
		return err
	}
	_, decoded.SummaryPresent = fields["summary"]
	_, decoded.ErrorPresent = fields["error"]
	_, decoded.MetaPresent = fields["_meta"]
	*v = SessionCompactionUpdate(decoded)
	return nil
}

func unmarshalCompactionPatch(data []byte, dst any) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return nil, err
	}
	return fields, nil
}

func marshalCompactionPatch(base any, summary []ContentBlock, detail *string, meta map[string]any, summaryPresent, errorPresent, metaPresent bool) ([]byte, error) {
	data, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for _, patch := range []struct {
		key     string
		value   any
		present bool
	}{
		{"summary", summary, summaryPresent || summary != nil},
		{"error", detail, errorPresent || detail != nil},
		{"_meta", meta, metaPresent || meta != nil},
	} {
		if !patch.present {
			continue
		}
		value, err := json.Marshal(patch.value)
		if err != nil {
			return nil, err
		}
		fields[patch.key] = value
	}
	return json.Marshal(fields)
}
