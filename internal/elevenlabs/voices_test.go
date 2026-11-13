package elevenlabs

import (
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

const (
	testSample     = "RIFF-fake-wav-sample"
	testClonedID   = "cloned-voice-id"
	testClonedName = "infer-0123456789abcdef"
)

func TestClonedVoiceName(t *testing.T) {
	name := ClonedVoiceName([]byte(testSample))

	assert.True(t, strings.HasPrefix(name, clonedVoicePrefix))
	assert.Len(t, name, len(clonedVoicePrefix)+clonedVoiceHexLength)
	assert.Equal(t, name, ClonedVoiceName([]byte(testSample)), "one sample always maps onto one voice")
	assert.NotEqual(t, name, ClonedVoiceName([]byte("another sample")))
}

func TestFindVoice(t *testing.T) {
	listed := `{"voices":[{"voice_id":"stock","name":"Brian"},{"voice_id":"` + testClonedID + `","name":"` + testClonedName + `"}]}`
	tests := []struct {
		name      string
		raw       string
		wantID    string
		wantFound bool
		wantErr   bool
	}{
		{name: "the voice is listed", raw: listed, wantID: testClonedID, wantFound: true},
		{name: "the voice is not listed", raw: `{"voices":[{"voice_id":"stock","name":"Brian"}]}`},
		{name: "no voices at all", raw: `{"voices":[]}`},
		{name: "not json", raw: `<html>`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, found, err := FindVoice([]byte(tt.raw), testClonedName)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantFound, found)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

func TestAddVoiceForm(t *testing.T) {
	contentType, body, err := AddVoiceForm(testClonedName, []byte(testSample))
	require.NoError(t, err)

	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	form, err := multipart.NewReader(strings.NewReader(string(body)), params["boundary"]).ReadForm(testFormMaxMemory)
	require.NoError(t, err)

	assert.Equal(t, []string{testClonedName}, form.Value[voiceNameField])
	require.Len(t, form.File[voiceFilesField], 1)
	file := form.File[voiceFilesField][0]
	assert.Equal(t, sampleContentType, file.Header.Get("Content-Type"))
	opened, err := file.Open()
	require.NoError(t, err)
	defer opened.Close()
	sample, err := io.ReadAll(opened)
	require.NoError(t, err)
	assert.Equal(t, testSample, string(sample), "the sample reaches elevenlabs byte for byte")
}

func TestAddedVoiceID(t *testing.T) {
	id, err := AddedVoiceID([]byte(`{"voice_id":"` + testClonedID + `","requires_verification":false}`))
	require.NoError(t, err)
	assert.Equal(t, testClonedID, id)

	_, err = AddedVoiceID([]byte(`{}`))
	require.Error(t, err)
}

func TestErrorMessage(t *testing.T) {
	assert.Equal(t, "missing the permission voices_read",
		ErrorMessage([]byte(`{"detail":{"status":"missing_permissions","message":"missing the permission voices_read"}}`)))
	assert.Equal(t, "bad gateway", ErrorMessage([]byte(" bad gateway\n")))
}
