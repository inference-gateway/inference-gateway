package providers

type GetModelsResponseGoogle struct {
	Models []interface{} `json:"models"`
}

type GenerateRequestGoogle struct {
	Contents struct{} `json:"contents"`
}

type GenerateResponseGoogle struct {
	Candidates []struct{} `json:"candidates"`
}
