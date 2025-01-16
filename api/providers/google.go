package providers

type GenerateRequestGoogleParts struct {
	Text string `json:"text"`
}

type GenerateRequestGoogleContents struct {
	Parts []GenerateRequestGoogleParts `json:"parts"`
}

type GenerateRequestGoogle struct {
	Contents GenerateRequestGoogleContents `json:"contents"`
}

type GenerateResponseGooglePart struct {
	Text string `json:"text"`
}

type GenerateResponseGoogleContent struct {
	Parts []GenerateResponseGooglePart `json:"parts"`
	Role  string                       `json:"role"`
}

type GenerateResponseGoogleCandidate struct {
	Content GenerateResponseGoogleContent `json:"content"`
}

type GenerateResponseGoogle struct {
	Candidates []GenerateResponseGoogleCandidate `json:"candidates"`
}
