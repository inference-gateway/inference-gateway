package elevenlabs

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	testSample         = "RIFF-fake-wav-sample"
	testOtherSample    = "RIFF-another-sample"
	testClonedVoice    = "cloned-voice-id"
	testPremadeName    = "Rachel"
	testRefusalBody    = `{"detail":{"status":"missing_permissions","message":"` + testRefusalMessage + `"}}`
	testRefusalMessage = "The API key you used is missing the permission voices_read to execute this operation."
	testPlainFailure   = "upstream is down"
	testBoundaryParam  = "boundary"
	testSpeakers       = 8
	notCloned          = 0
	clonedOnce         = 1
)

// fakeAccount stands in for an ElevenLabs account: it lists the voices it
// holds and keeps the voice a clone request creates.
type fakeAccount struct {
	mu          sync.Mutex
	voices      map[string]string
	clones      int
	clonedFrom  string
	listRefusal error
	listGate    chan struct{}
}

func (a *fakeAccount) ListVoices(ctx context.Context) ([]byte, error) {
	if a.listGate != nil {
		<-a.listGate
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.listRefusal != nil {
		return nil, a.listRefusal
	}
	type voice struct {
		VoiceID string `json:"voice_id"`
		Name    string `json:"name"`
	}
	listed := []voice{}
	for name, id := range a.voices {
		listed = append(listed, voice{VoiceID: id, Name: name})
	}
	return json.Marshal(map[string]any{"voices": listed})
}

func (a *fakeAccount) CreateVoice(ctx context.Context, contentType string, form []byte) ([]byte, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, err
	}
	parsed, err := multipart.NewReader(strings.NewReader(string(form)), params[testBoundaryParam]).ReadForm(testFormMaxMemory)
	if err != nil {
		return nil, err
	}
	file, err := parsed.File[voiceFilesField][0].Open()
	if err != nil {
		return nil, err
	}
	sample, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.clones++
	a.clonedFrom = string(sample)
	if a.voices == nil {
		a.voices = map[string]string{}
	}
	a.voices[parsed.Value[voiceNameField][0]] = testClonedVoice
	return json.Marshal(map[string]any{"voice_id": testClonedVoice})
}

func speechInSample(sample string) types.CreateSpeechRequest {
	return types.CreateSpeechRequest{Voice: testVoice, ReferenceAudio: ptr([]byte(sample))}
}

func TestInSampleVoice(t *testing.T) {
	clonedBefore := map[string]string{VoiceSample(testSample).voiceName(): testClonedVoice}
	tests := []struct {
		name       string
		req        types.CreateSpeechRequest
		voices     map[string]string
		wantVoice  string
		wantClones int
	}{
		{"a request without a sample keeps the voice it names", types.CreateSpeechRequest{Voice: testVoice}, nil, testVoice, notCloned},
		{"a new sample is cloned into the account", speechInSample(testSample), map[string]string{testPremadeName: testVoice}, testClonedVoice, clonedOnce},
		{"a sample cloned before is found, not cloned again", speechInSample(testSample), clonedBefore, testClonedVoice, notCloned},
		{"another sample is another voice", speechInSample(testOtherSample), clonedBefore, testClonedVoice, clonedOnce},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &fakeAccount{voices: tt.voices}
			var library VoiceLibrary

			got, err := library.InSampleVoice(t.Context(), account, tt.req)

			require.NoError(t, err)
			assert.Equal(t, tt.wantVoice, got.Voice)
			assert.Nil(t, got.ReferenceAudio, "the sample is not sent with the speech call")
			assert.Equal(t, tt.wantClones, account.clones)
		})
	}
}

func TestInSampleVoice_SendsTheSampleByteForByte(t *testing.T) {
	account := &fakeAccount{}
	var library VoiceLibrary

	_, err := library.InSampleVoice(t.Context(), account, speechInSample(testSample))

	require.NoError(t, err)
	assert.Equal(t, testSample, account.clonedFrom)
	assert.Contains(t, account.voices, VoiceSample(testSample).voiceName())
}

func TestInSampleVoice_ReportsARefusal(t *testing.T) {
	account := &fakeAccount{listRefusal: NewRefusal(http.StatusUnauthorized, []byte(testRefusalBody))}
	var library VoiceLibrary

	_, err := library.InSampleVoice(t.Context(), account, speechInSample(testSample))

	var refusal *Refusal
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, http.StatusUnauthorized, refusal.Status)
	assert.Contains(t, err.Error(), testRefusalMessage)
	assert.Zero(t, account.clones, "nothing is cloned once the lookup is refused")
}

func TestInSampleVoice_ClonesASampleOnceForManySpeakers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		account := &fakeAccount{listGate: make(chan struct{})}
		var library VoiceLibrary
		voices := make([]string, testSpeakers)

		var speakers sync.WaitGroup
		for i := range testSpeakers {
			speakers.Go(func() {
				got, err := library.InSampleVoice(t.Context(), account, speechInSample(testSample))
				assert.NoError(t, err)
				voices[i] = got.Voice
			})
		}
		synctest.Wait()
		close(account.listGate)
		speakers.Wait()

		assert.Equal(t, clonedOnce, account.clones)
		for _, voice := range voices {
			assert.Equal(t, testClonedVoice, voice)
		}
	})
}

func TestInSampleVoice_OutlivesASpeakerThatGivesUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		account := &fakeAccount{listGate: make(chan struct{})}
		var library VoiceLibrary
		impatient, giveUp := context.WithCancel(t.Context())

		var speakers sync.WaitGroup
		var impatientErr error
		speakers.Go(func() {
			_, impatientErr = library.InSampleVoice(impatient, account, speechInSample(testSample))
		})
		synctest.Wait()

		var patient types.CreateSpeechRequest
		var patientErr error
		speakers.Go(func() {
			patient, patientErr = library.InSampleVoice(t.Context(), account, speechInSample(testSample))
		})
		synctest.Wait()

		giveUp()
		synctest.Wait()
		close(account.listGate)
		speakers.Wait()

		assert.ErrorIs(t, impatientErr, context.Canceled)
		require.NoError(t, patientErr)
		assert.Equal(t, testClonedVoice, patient.Voice)
		assert.Equal(t, clonedOnce, account.clones)
	})
}

func TestVoiceName_IsStablePerSample(t *testing.T) {
	name := VoiceSample(testSample).voiceName()

	assert.Equal(t, name, VoiceSample(testSample).voiceName())
	assert.NotEqual(t, name, VoiceSample(testOtherSample).voiceName())
	assert.True(t, strings.HasPrefix(name, clonedVoicePrefix))
	assert.Len(t, name, len(clonedVoicePrefix)+clonedVoiceHexLength)
}

func TestNewRefusal(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"elevenlabs' own message", testRefusalBody, testRefusalMessage},
		{"a body that is not an elevenlabs error", " " + testPlainFailure + "\n", testPlainFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refusal := NewRefusal(http.StatusBadGateway, []byte(tt.body))

			assert.Equal(t, http.StatusBadGateway, refusal.Status)
			assert.Equal(t, tt.want, refusal.Error())
		})
	}
}
