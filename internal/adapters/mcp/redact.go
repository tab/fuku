package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// tokenRedacted replaces the API token wherever a tool result or error holds it
const tokenRedacted = "[redacted]"

// redactCalls removes the API tokens of the call's project from every tool result and error before the agent sees it
func (s *Server) redactCalls(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		params, ok := req.GetParams().(*sdk.CallToolParamsRaw)
		if method != methodCallTool || !ok {
			return next(ctx, method, req)
		}

		result, err := next(ctx, method, req)

		tokens := s.tokens(params.Arguments)
		if len(tokens) == 0 {
			return result, err
		}

		if call, ok := result.(*sdk.CallToolResult); ok && call != nil {
			redactResult(call, tokens)
		}

		return result, redactError(err, tokens)
	}
}

// tokens loads the API tokens of the config the call names and of its project's default lookup, as either may run
func (s *Server) tokens(arguments json.RawMessage) []string {
	var in target

	//nolint:errcheck // a call with unreadable arguments names no config, so the default lookup applies
	json.Unmarshal(arguments, &in)

	dir, err := projectDir(in.Config)
	names := slices.Compact([]string{configName(in.Config), ""})

	if err != nil {
		dir, err = projectDir("")
		names = []string{""}
	}

	if err != nil {
		return nil
	}

	var tokens []string

	for _, name := range names {
		token := s.load(dir, name).Project.Server.Token
		if token != "" && !slices.Contains(tokens, token) {
			tokens = append(tokens, token)
		}
	}

	return tokens
}

// redactResult replaces the tokens in the text and structured content, dropping a structured content it cannot redact
func redactResult(call *sdk.CallToolResult, tokens []string) {
	for _, content := range call.Content {
		if text, ok := content.(*sdk.TextContent); ok {
			text.Text = redact(text.Text, tokens)
		}
	}

	if call.StructuredContent == nil {
		return
	}

	raw, err := json.Marshal(call.StructuredContent)
	redacted := redact(string(raw), tokens)

	if err == nil && redacted == string(raw) {
		return
	}

	var structured any
	if err != nil || json.Unmarshal([]byte(redacted), &structured) != nil {
		call.StructuredContent = nil

		return
	}

	call.StructuredContent = structured
}

// redactError rewrites an error only when its text holds a token, keeping a JSON-RPC error with its code
func redactError(err error, tokens []string) error {
	if err == nil || redact(err.Error(), tokens) == err.Error() {
		return err
	}

	if wire, ok := errors.AsType[*jsonrpc.Error](err); ok {
		return &jsonrpc.Error{Code: wire.Code, Message: redact(wire.Message, tokens), Data: wire.Data}
	}

	return errors.New(redact(err.Error(), tokens))
}

// redact replaces every token in a text, both as written and as a JSON string escapes it
func redact(value string, tokens []string) string {
	for _, token := range tokens {
		escaped, _ := json.Marshal(token)
		value = strings.ReplaceAll(value, token, tokenRedacted)
		value = strings.ReplaceAll(value, string(escaped[1:len(escaped)-1]), tokenRedacted)
	}

	return value
}
