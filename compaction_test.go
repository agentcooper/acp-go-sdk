package acp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Exercise identical patch semantics through the standalone type, the flattened
// session variant, and the discriminator-based SessionUpdate union.
func TestCompactionUpdate_Patch(t *testing.T) {
	for _, kind := range []string{"standalone", "session", "union"} {
		t.Run(kind, func(t *testing.T) {
			for _, tc := range []struct {
				name  string
				patch string
				want  CompactionUpdate
			}{
				{name: "missing"},
				{name: "null", patch: `,"summary":null,"error":null,"_meta":null`, want: CompactionUpdate{SummaryPresent: true, ErrorPresent: true, MetaPresent: true}},
				{name: "empty", patch: `,"summary":[],"error":"","_meta":{}`, want: CompactionUpdate{Summary: []ContentBlock{}, Error: Ptr(""), Meta: map[string]any{}, SummaryPresent: true, ErrorPresent: true, MetaPresent: true}},
				{name: "concrete", patch: `,"summary":[{"type":"text","text":"retained"}],"error":"failed","_meta":{"source":"test"}`, want: CompactionUpdate{Summary: []ContentBlock{TextBlock("retained")}, Error: Ptr("failed"), Meta: map[string]any{"source": "test"}, SummaryPresent: true, ErrorPresent: true, MetaPresent: true}},
				{name: "summary only", patch: `,"summary":null`, want: CompactionUpdate{SummaryPresent: true}},
				{name: "error only", patch: `,"error":null`, want: CompactionUpdate{ErrorPresent: true}},
				{name: "meta only", patch: `,"_meta":null`, want: CompactionUpdate{MetaPresent: true}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					input := `{"compactionId":"c1","status":"in_progress"` + tc.patch
					if kind != "standalone" {
						input += `,"sessionUpdate":"compaction_update"`
					}
					input += `}`
					tc.want.CompactionId = "c1"
					tc.want.Status = "in_progress"
					got, encoded := compactionPatchRoundTrip(t, kind, []byte(input))
					require.Equal(t, tc.want, got)
					require.JSONEq(t, input, string(encoded))
				})
			}
		})
	}
}

func compactionPatchRoundTrip(t *testing.T, kind string, input []byte) (CompactionUpdate, []byte) {
	t.Helper()
	var got CompactionUpdate
	var value any
	switch kind {
	case "standalone":
		require.NoError(t, json.Unmarshal(input, &got))
		value = got
	case "session", "union":
		var session SessionCompactionUpdate
		if kind == "session" {
			require.NoError(t, json.Unmarshal(input, &session))
			value = session
		} else {
			var update SessionUpdate
			require.NoError(t, json.Unmarshal(input, &update))
			require.NotNil(t, update.CompactionUpdate)
			session = *update.CompactionUpdate
			value = update
		}
		got = CompactionUpdate{
			CompactionId: session.CompactionId, Status: session.Status,
			Summary: session.Summary, Error: session.Error, Meta: session.Meta,
			SummaryPresent: session.SummaryPresent, ErrorPresent: session.ErrorPresent, MetaPresent: session.MetaPresent,
		}
	}
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return got, encoded
}

func TestCompactionUpdate_MarshalPatch(t *testing.T) {
	for _, tc := range []struct {
		name  string
		patch CompactionUpdate
		want  string
	}{
		{name: "omitted", want: `{}`},
		{name: "explicit null", patch: CompactionUpdate{SummaryPresent: true, ErrorPresent: true, MetaPresent: true}, want: `{"summary":null,"error":null,"_meta":null}`},
		{name: "empty values without flags", patch: CompactionUpdate{Summary: []ContentBlock{}, Error: Ptr(""), Meta: map[string]any{}}, want: `{"summary":[],"error":"","_meta":{}}`},
		{name: "values without flags", patch: CompactionUpdate{Summary: []ContentBlock{TextBlock("kept")}, Error: Ptr("oops"), Meta: map[string]any{"x": true}}, want: `{"summary":[{"type":"text","text":"kept"}],"error":"oops","_meta":{"x":true}}`},
	} {
		for _, kind := range []string{"standalone", "session", "union"} {
			t.Run(tc.name+"/"+kind, func(t *testing.T) {
				p := tc.patch
				var value any = p
				if kind != "standalone" {
					session := SessionCompactionUpdate{Summary: p.Summary, Error: p.Error, Meta: p.Meta, SummaryPresent: p.SummaryPresent, ErrorPresent: p.ErrorPresent, MetaPresent: p.MetaPresent}
					if kind == "session" {
						value = session
					} else {
						value = SessionUpdate{CompactionUpdate: &session}
					}
				}
				data, err := json.Marshal(value)
				require.NoError(t, err)
				var fields map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(data, &fields))
				delete(fields, "compactionId")
				delete(fields, "status")
				delete(fields, "sessionUpdate")
				data, err = json.Marshal(fields)
				require.NoError(t, err)
				require.JSONEq(t, tc.want, string(data))
			})
		}
	}
}

func TestCompactionUpdate_DecodeReuse(t *testing.T) {
	for _, dst := range []any{&CompactionUpdate{}, &SessionCompactionUpdate{}} {
		require.NoError(t, json.Unmarshal([]byte(`{"summary":[],"error":"old","_meta":{"old":true}}`), dst))
		require.NoError(t, json.Unmarshal([]byte(`{}`), dst))
		data, err := json.Marshal(dst)
		require.NoError(t, err)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &fields))
		for _, key := range []string{"summary", "error", "_meta"} {
			require.NotContains(t, fields, key)
		}
		for _, invalid := range []string{`{"summary":false}`, `{"error":[]}`, `{"_meta":[]}`, `{"summary":`} {
			require.Error(t, json.Unmarshal([]byte(invalid), dst))
		}
	}
}

func TestCompactionUpdate_MarshalError(t *testing.T) {
	for _, value := range []any{
		CompactionUpdate{Meta: map[string]any{"bad": make(chan int)}},
		SessionCompactionUpdate{Meta: map[string]any{"bad": make(chan int)}},
	} {
		_, err := json.Marshal(value)
		require.Error(t, err)
	}
}
