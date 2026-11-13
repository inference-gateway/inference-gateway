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
	testSample      = "RIFF-fake-wav-sample"
	testOtherSample = "RIFF-another-sample"
	testClonedVoice = "cloned-voice-id"
	testRefusal     = "The API key you used is missing the permission voices_read to execute this operation."
)

func TestClonedVoiceName_IsStablePerSample(t *testing.T) {
	name := ClonedVoiceName([]byte(testSample))

	assert.Equal(t, name, ClonedVoiceName([]byte(testSample)))
	assert.NotEqual(t, name, ClonedVoiceName([]byte(testOtherSample)))
	assert.True(t, strings.HasPrefix(name, clonedVoicePrefix))
	assert.Len(t, name, len(clonedVoicePrefix)+clonedVoiceHexLength)
}

func TestFindVoice(t *testing.T) {
	name := ClonedVoiceName([]byte(testSample))
	listed := []byte(`{"voices":[{"voice_id":"premade","name":"Rachel"},{"voice_id":"` + testClonedVoice + `","name":"` + name + `"}]}`)

	id, err := FindVoice(listed, name)
	require.NoError(t, err)
	assert.Equal(t, testClonedVoice, id)

	id, err = FindVoice(listed, ClonedVoiceName([]byte(testOtherSample)))
	require.NoError(t, err)
	assert.Empty(t, id, "a sample never cloned has no voice")

	_, err = FindVoice([]byte("not json"), name)
	assert.Error(t, err)
}

func TestCreateVoiceForm_CarriesTheNameAndTheSample(t *testing.T) {
	name := ClonedVoiceName([]byte(testSample))

	contentType, body, err := CreateVoiceForm(name, []byte(testSample))
	require.NoError(t, err)

	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	form, err := multipart.NewReader(strings.NewReader(string(body)), params["boundary"]).ReadForm(testFormMaxMemory)
	require.NoError(t, err)
	assert.Equal(t, []string{name}, form.Value[voiceNameField])
	require.Len(t, form.File[voiceFilesField], 1)
	file, err := form.File[voiceFilesField][0].Open()
	require.NoError(t, err)
	sample, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, testSample, string(sample))
}

func TestCreatedVoiceID(t *testing.T) {
	id, err := CreatedVoiceID([]byte(`{"voice_id":"` + testClonedVoice + `","requires_verification":false}`))
	require.NoError(t, err)
	assert.Equal(t, testClonedVoice, id)

	_, err = CreatedVoiceID([]byte(`{}`))
	assert.Error(t, err, "a response without a voice id cannot be spoken in")
}

func TestErrorMessage(t *testing.T) {
	assert.Equal(t, testRefusal, ErrorMessage([]byte(`{"detail":{"status":"missing_permissions","message":"`+testRefusal+`"}}`)))
	assert.Equal(t, "upstream is down", ErrorMessage([]byte(" upstream is down\n")))
}
