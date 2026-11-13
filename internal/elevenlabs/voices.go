package elevenlabs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"

	singleflight "golang.org/x/sync/singleflight"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	clonedVoicePrefix    = "infer-"
	clonedVoiceHexLength = 16

	voiceNameField           = "name"
	voiceFilesField          = "files"
	sampleFileName           = "sample.wav"
	sampleContentType        = "audio/wav"
	contentTypeHeader        = "Content-Type"
	contentDispositionHeader = "Content-Disposition"
	sampleDispositionFormat  = `form-data; name=%q; filename=%q`
)

// Account is the ElevenLabs account behind the gateway's API key. Each call
// returns the response body, or a *Refusal when ElevenLabs declines it.
type Account interface {
	ListVoices(ctx context.Context) ([]byte, error)
	CreateVoice(ctx context.Context, contentType string, form []byte) ([]byte, error)
}

// Refusal is ElevenLabs declining a call, with the status it answered and its
// own message, so a key without the voices permissions says so.
type Refusal struct {
	Status  int
	Message string
}

// NewRefusal reads the readable message out of an ElevenLabs error body,
// falling back to the body itself.
func NewRefusal(status int, body []byte) *Refusal {
	var declined struct {
		Detail struct {
			Message string `json:"message"`
		} `json:"detail"`
	}
	if err := json.Unmarshal(body, &declined); err == nil && declined.Detail.Message != "" {
		return &Refusal{Status: status, Message: declined.Detail.Message}
	}
	return &Refusal{Status: status, Message: strings.TrimSpace(string(body))}
}

func (r *Refusal) Error() string { return r.Message }

// VoiceSample is a reference recording of the speaker a request wants to
// sound like.
type VoiceSample []byte

// voiceName names the voice cloned from the sample after its content, so one
// sample always maps onto one voice of the account.
func (s VoiceSample) voiceName() string {
	sum := sha256.Sum256(s)
	return clonedVoicePrefix + hex.EncodeToString(sum[:])[:clonedVoiceHexLength]
}

// cloneForm builds the multipart body that clones the sample into a voice
// called name.
func (s VoiceSample) cloneForm(name string) (contentType string, form []byte, err error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField(voiceNameField, name); err != nil {
		return "", nil, err
	}
	header := textproto.MIMEHeader{}
	header.Set(contentDispositionHeader, fmt.Sprintf(sampleDispositionFormat, voiceFilesField, sampleFileName))
	header.Set(contentTypeHeader, sampleContentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", nil, err
	}
	if _, err := part.Write(s); err != nil {
		return "", nil, err
	}
	if err := writer.Close(); err != nil {
		return "", nil, err
	}
	return writer.FormDataContentType(), body.Bytes(), nil
}

// VoiceLibrary keeps one cloned voice per voice sample in an ElevenLabs
// account. Its zero value is ready to use.
type VoiceLibrary struct {
	lookups singleflight.Group
}

// InSampleVoice returns req speaking in the voice cloned from its sample
// instead of the voice it names, since ElevenLabs takes no sample with a
// speech request. A request without a sample is returned as it is.
func (l *VoiceLibrary) InSampleVoice(ctx context.Context, account Account, req types.CreateSpeechRequest) (types.CreateSpeechRequest, error) {
	if req.ReferenceAudio == nil || len(*req.ReferenceAudio) == 0 {
		return req, nil
	}
	voice, err := l.voiceOf(ctx, account, VoiceSample(*req.ReferenceAudio))
	if err != nil {
		return req, err
	}
	req.Voice, req.ReferenceAudio = voice, nil
	return req, nil
}

// voiceOf returns the id of the voice cloned from sample. Callers with the
// same sample share one lookup, so two sentences in a new voice never clone
// it twice. The lookup outlives a caller that gives up, because others may be
// waiting on it.
func (l *VoiceLibrary) voiceOf(ctx context.Context, account Account, sample VoiceSample) (string, error) {
	name := sample.voiceName()
	lookup := l.lookups.DoChan(name, func() (any, error) {
		return findOrClone(context.WithoutCancel(ctx), account, name, sample)
	})

	select {
	case found := <-lookup:
		voice, _ := found.Val.(string)
		return voice, found.Err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// findOrClone asks the account for the voice called name and clones sample
// into it when the account has none. The account is asked every time, so a
// voice deleted at ElevenLabs is cloned again.
func findOrClone(ctx context.Context, account Account, name string, sample VoiceSample) (string, error) {
	listed, err := account.ListVoices(ctx)
	if err != nil {
		return "", fmt.Errorf("listing the elevenlabs voices: %w", err)
	}
	voice, err := voiceCalled(name, listed)
	if err != nil || voice != "" {
		return voice, err
	}

	contentType, form, err := sample.cloneForm(name)
	if err != nil {
		return "", fmt.Errorf("building the voice clone request: %w", err)
	}
	created, err := account.CreateVoice(ctx, contentType, form)
	if err != nil {
		return "", fmt.Errorf("cloning the voice sample: %w", err)
	}
	return createdVoice(created)
}

// voiceCalled returns the id of the voice called name in a list-voices
// response, or "" when the account has none.
func voiceCalled(name string, listed []byte) (string, error) {
	var account struct {
		Voices []struct {
			VoiceID string `json:"voice_id"`
			Name    string `json:"name"`
		} `json:"voices"`
	}
	if err := json.Unmarshal(listed, &account); err != nil {
		return "", fmt.Errorf("decoding the elevenlabs voice list: %w", err)
	}
	for _, voice := range account.Voices {
		if voice.Name == name {
			return voice.VoiceID, nil
		}
	}
	return "", nil
}

// createdVoice reads the id of the voice a create-voice response reports.
func createdVoice(created []byte) (string, error) {
	var voice struct {
		VoiceID string `json:"voice_id"`
	}
	if err := json.Unmarshal(created, &voice); err != nil {
		return "", fmt.Errorf("decoding the cloned elevenlabs voice: %w", err)
	}
	if voice.VoiceID == "" {
		return "", fmt.Errorf("elevenlabs returned no voice id for the cloned voice")
	}
	return voice.VoiceID, nil
}
