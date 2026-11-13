package elevenlabs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"
)

const (
	VoicesEndpoint   = "/voices"
	AddVoiceEndpoint = "/voices/add"

	clonedVoicePrefix    = "infer-"
	clonedVoiceHexLength = 16

	voiceNameField    = "name"
	voiceFilesField   = "files"
	sampleFileName    = "sample.wav"
	sampleContentType = "audio/wav"
)

// ClonedVoiceName names the instant voice clone of a reference recording after
// its content, so one sample always maps onto one ElevenLabs voice.
func ClonedVoiceName(sample []byte) string {
	sum := sha256.Sum256(sample)
	return clonedVoicePrefix + hex.EncodeToString(sum[:])[:clonedVoiceHexLength]
}

type voiceList struct {
	Voices []struct {
		VoiceID string `json:"voice_id"`
		Name    string `json:"name"`
	} `json:"voices"`
}

// FindVoice returns the id of the voice called name in a GET /voices response.
func FindVoice(raw []byte, name string) (id string, found bool, err error) {
	var listed voiceList
	if err := json.Unmarshal(raw, &listed); err != nil {
		return "", false, fmt.Errorf("failed to decode the elevenlabs voice list: %w", err)
	}
	for _, voice := range listed.Voices {
		if voice.Name == name {
			return voice.VoiceID, true, nil
		}
	}
	return "", false, nil
}

// AddVoiceForm builds the POST /voices/add multipart body that clones sample
// into a voice called name.
func AddVoiceForm(name string, sample []byte) (contentType string, body []byte, err error) {
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	if err := writer.WriteField(voiceNameField, name); err != nil {
		return "", nil, err
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, voiceFilesField, sampleFileName))
	header.Set("Content-Type", sampleContentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", nil, err
	}
	if _, err := part.Write(sample); err != nil {
		return "", nil, err
	}
	if err := writer.Close(); err != nil {
		return "", nil, err
	}
	return writer.FormDataContentType(), form.Bytes(), nil
}

// AddedVoiceID reads the id of the voice a POST /voices/add response created.
func AddedVoiceID(raw []byte) (string, error) {
	var added struct {
		VoiceID string `json:"voice_id"`
	}
	if err := json.Unmarshal(raw, &added); err != nil {
		return "", fmt.Errorf("failed to decode the cloned elevenlabs voice: %w", err)
	}
	if added.VoiceID == "" {
		return "", fmt.Errorf("elevenlabs returned no voice id for the cloned voice")
	}
	return added.VoiceID, nil
}

// ErrorMessage pulls the readable message out of an ElevenLabs error body,
// falling back to the body itself.
func ErrorMessage(raw []byte) string {
	var failure struct {
		Detail struct {
			Message string `json:"message"`
		} `json:"detail"`
	}
	if err := json.Unmarshal(raw, &failure); err == nil && failure.Detail.Message != "" {
		return failure.Detail.Message
	}
	return strings.TrimSpace(string(raw))
}
