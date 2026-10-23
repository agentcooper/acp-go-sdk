package acp

import "fmt"

// validateElicitation checks the flattened scope that cannot be represented by
// the generated outer mode union alone.
func (u *CreateElicitationRequest) validateElicitation() error {
	if u == nil {
		return fmt.Errorf("elicitation request must not be nil")
	}
	count := 0
	for _, present := range []bool{u.Form != nil, u.Url != nil, u.Other != nil} {
		if present {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("elicitation requires exactly one mode variant")
	}
	var session *SessionId
	var request *RequestId
	var tool *ToolCallId
	if p := u.Form; p != nil {
		if p.Mode != "form" {
			return fmt.Errorf("elicitation mode must be explicitly set to form")
		}
		session, request, tool = p.SessionId, p.RequestId, p.ToolCallId
	} else if p := u.Url; p != nil {
		if p.Mode != "url" {
			return fmt.Errorf("elicitation mode must be explicitly set to url")
		}
		session, request, tool = p.SessionId, p.RequestId, p.ToolCallId
	} else if p := u.Other; p != nil {
		if p.Mode == "" || p.Mode == "form" || p.Mode == "url" {
			return fmt.Errorf("invalid other elicitation mode %q", p.Mode)
		}
		session, request, tool = p.SessionId, p.RequestId, p.ToolCallId
	}
	if (session == nil) == (request == nil) {
		return fmt.Errorf("elicitation requires exactly one of sessionId or requestId")
	}
	if tool != nil && session == nil {
		return fmt.Errorf("elicitation toolCallId requires sessionId")
	}
	if request != nil && (request.Null != nil || (request.Number == nil) == (request.Str == nil)) {
		return fmt.Errorf("elicitation requestId must be a string or number")
	}
	return nil
}
