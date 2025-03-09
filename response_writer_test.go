package estellm_test

import (
	"bytes"
	"testing"

	"github.com/mashiike/estellm"
	"github.com/stretchr/testify/require"
)

func TestBatchResponseWriter(t *testing.T) {
	writer := estellm.NewBatchResponseWriter()

	err := writer.WriteRole(estellm.RoleAssistant)
	require.NoError(t, err)
	err = writer.WritePart(estellm.ReasoningPart("Reason"), estellm.ReasoningPart(" Say"))
	require.NoError(t, err)

	err = writer.WritePart(estellm.TextPart("Hello"), estellm.TextPart(" World"))
	require.NoError(t, err)

	err = writer.WritePart(estellm.TextPart("!"))
	require.NoError(t, err)

	// Test Finish
	err = writer.Finish(estellm.FinishReasonEndTurn, "Finished")
	require.NoError(t, err)

	// Test Response
	resp := writer.Response()
	require.NotNil(t, resp)
	require.EqualValues(t, estellm.FinishReasonEndTurn, resp.FinishReason)
	require.Equal(t, "Finished", resp.FinishMessage)
	require.Equal(t, estellm.RoleAssistant, resp.Message.Role)
	require.Len(t, resp.Message.Parts, 2)
	require.Equal(t, estellm.PartTypeReasoning, resp.Message.Parts[0].Type)
	require.Equal(t, "Reason Say", resp.Message.Parts[0].Text)
	require.Equal(t, estellm.PartTypeText, resp.Message.Parts[1].Type)
	require.Equal(t, "Hello World!", resp.Message.Parts[1].Text)
}

func TestTextStreamingResponseWriter(t *testing.T) {
	var buf bytes.Buffer
	writer := estellm.NewTextStreamingResponseWriter(&buf)

	// Test WriteRole (no-op)
	err := writer.WriteRole(estellm.RoleAssistant)
	require.NoError(t, err)

	// Test WritePart
	err = writer.WritePart(estellm.TextPart("Hello"), estellm.TextPart(" World"))
	require.NoError(t, err)
	require.Equal(t, "Hello World", buf.String())

	// Test Finish
	err = writer.Finish(estellm.FinishReasonEndTurn, "Finished")
	require.NoError(t, err)

	// Test DumpMetadata
	writer.DumpMetadata()
	require.Contains(t, buf.String(), "Finish-Reason: end_turn")
	require.Contains(t, buf.String(), "Finish-Message: Finished")
}
