package acp

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestElicitation_RequestRoundTrip(t *testing.T) {
	for _, mode := range []string{"form", "url"} {
		for _, scope := range []string{`"sessionId":"s"`, `"sessionId":"s","toolCallId":"t"`, `"requestId":0`, `"requestId":42`, `"requestId":"initialize-1"`} {
			t.Run(mode+scope, func(t *testing.T) {
				fields := `"requestedSchema":{"type":"object","properties":{"strategy":{"type":"string","enum":["balanced","aggressive"]}},"required":["strategy"]}`
				if mode == "url" {
					fields = `"elicitationId":"oauth-1","url":"https://example.com/connect"`
				}
				payload := `{"mode":"` + mode + `","message":"Choose",` + scope + `,` + fields + `,"_meta":{"test":true}}`
				var request CreateElicitationRequest
				require.NoError(t, json.Unmarshal([]byte(payload), &request))
				require.NoError(t, request.Validate())
				encoded, err := json.Marshal(request)
				require.NoError(t, err)
				require.JSONEq(t, payload, string(encoded))
			})
		}
	}
}

func TestElicitation_InvalidRequest(t *testing.T) {
	for _, mode := range []string{"form", "url"} {
		for _, scope := range []string{
			``, `,"sessionId":null`, `,"requestId":null`,
			`,"sessionId":"s","requestId":0`,
			`,"requestId":1,"toolCallId":"t"`,
			`,"toolCallId":"t"`, `,"requestId":{}`, `,"requestId":true`,
		} {
			t.Run(mode+scope, func(t *testing.T) {
				payload := `{"mode":"` + mode + `","message":"Choose","requestedSchema":{},"elicitationId":"e","url":"https://example.com"` + scope + `}`
				c := &ClientSideConnection{client: &elicitationClient{}}
				_, err := c.handle(context.Background(), ClientMethodElicitationCreate, json.RawMessage(payload))
				require.NotNil(t, err)
				require.Equal(t, -32602, err.Code)
			})
		}
	}
}

func TestElicitation_Validate(t *testing.T) {
	for _, request := range []*CreateElicitationRequest{
		nil, {},
		{Form: &CreateElicitationForm{}, Url: &CreateElicitationUrl{}},
		{Form: &CreateElicitationForm{SessionId: Ptr(SessionId("s"))}},
		{Url: &CreateElicitationUrl{Mode: "form", SessionId: Ptr(SessionId("s"))}},
		{Other: &CreateElicitationOther{Mode: "form", SessionId: Ptr(SessionId("s"))}},
		{Form: &CreateElicitationForm{Mode: "form", RequestId: &RequestId{}}},
		{Form: &CreateElicitationForm{Mode: "form", RequestId: &RequestId{Number: Ptr(RequestIdNumber(0)), Str: Ptr(RequestIdStr("r"))}}},
	} {
		require.Error(t, request.Validate())
	}
	for _, mode := range []string{"_custom", "future"} {
		var request CreateElicitationRequest
		payload := `{"mode":"` + mode + `","message":"Choose","sessionId":"s","toolCallId":"t"}`
		require.NoError(t, json.Unmarshal([]byte(payload), &request))
		require.NotNil(t, request.Other)
		require.NoError(t, request.Validate())
		encoded, err := json.Marshal(request)
		require.NoError(t, err)
		require.JSONEq(t, payload, string(encoded))
	}
}

func TestElicitation_ResponseRoundTrip(t *testing.T) {
	for _, payload := range []string{
		`{"action":"accept"}`,
		`{"action":"accept","content":{"name":"Ada","count":2,"score":1.5,"enabled":false,"choices":["a","b"]}}`,
		`{"action":"decline"}`, `{"action":"cancel"}`,
	} {
		var response CreateElicitationResponse
		require.NoError(t, json.Unmarshal([]byte(payload), &response))
		require.NoError(t, response.Validate())
		encoded, err := json.Marshal(response)
		require.NoError(t, err)
		require.JSONEq(t, payload, string(encoded))
	}
	for _, payload := range []string{`{"action":"accept","content":null}`, `{"action":"decline","content":{"ignored":true}}`, `{"action":"cancel","content":{"ignored":true}}`} {
		var response CreateElicitationResponse
		require.NoError(t, json.Unmarshal([]byte(payload), &response))
		encoded, err := json.Marshal(response)
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, json.Unmarshal(encoded, &fields))
		require.NotContains(t, fields, "content")
	}
}

type elicitationClient struct {
	Client
	received  chan CreateElicitationRequest
	completed chan CompleteElicitationNotification
}

func (c *elicitationClient) CreateElicitation(_ context.Context, p CreateElicitationRequest) (CreateElicitationResponse, error) {
	c.received <- p
	return CreateElicitationResponse{Accept: &CreateElicitationAccept{}}, nil
}

func (c *elicitationClient) CompleteElicitation(_ context.Context, p CompleteElicitationNotification) error {
	c.completed <- p
	return nil
}

func TestElicitation_Connection(t *testing.T) {
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() { c2aR.Close(); c2aW.Close(); a2cR.Close(); a2cW.Close() })
	client := &elicitationClient{received: make(chan CreateElicitationRequest, 1), completed: make(chan CompleteElicitationNotification, 1)}
	NewClientSideConnection(client, c2aW, a2cR)
	agent := NewAgentSideConnection(nil, a2cW, c2aR)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	response, err := agent.CreateElicitation(ctx, CreateElicitationRequest{Url: &CreateElicitationUrl{
		RequestId: &RequestId{Number: Ptr(RequestIdNumber(0))}, Message: "Authorize", ElicitationId: "oauth-1", Url: "https://example.com/connect",
	}})
	require.NoError(t, err)
	require.NotNil(t, response.Accept)
	select {
	case p := <-client.received:
		require.Equal(t, RequestIdNumber(0), *p.Url.RequestId.Number)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	response, err = agent.CreateElicitation(ctx, CreateElicitationRequest{Form: &CreateElicitationForm{
		SessionId: Ptr(SessionId("session-1")), ToolCallId: Ptr(ToolCallId("tool-1")),
		Message: "Choose", RequestedSchema: ElicitationSchema{Properties: map[string]any{}},
	}})
	require.NoError(t, err)
	require.NotNil(t, response.Accept)
	select {
	case p := <-client.received:
		require.Equal(t, SessionId("session-1"), *p.Form.SessionId)
		require.Equal(t, ToolCallId("tool-1"), *p.Form.ToolCallId)
		require.Nil(t, p.Form.RequestId)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, agent.CompleteElicitation(ctx, CompleteElicitationNotification{ElicitationId: "oauth-1"}))
	select {
	case p := <-client.completed:
		require.Equal(t, ElicitationId("oauth-1"), p.ElicitationId)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestElicitation_Capabilities(t *testing.T) {
	for _, payload := range []string{`{}`, `{"elicitation":null}`, `{"elicitation":{}}`, `{"elicitation":{"form":null,"url":null}}`, `{"elicitation":{"form":{}}}`, `{"elicitation":{"url":{}}}`, `{"elicitation":{"form":{},"url":{}}}`} {
		t.Run(payload, func(t *testing.T) {
			var caps ClientCapabilities
			require.NoError(t, json.Unmarshal([]byte(payload), &caps))
			encoded, err := json.Marshal(caps)
			require.NoError(t, err)
			var decoded ClientCapabilities
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			require.Equal(t, caps.Elicitation, decoded.Elicitation)
		})
	}
}
