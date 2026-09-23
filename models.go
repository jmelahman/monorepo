package typesafe

import "context"

const modelsPath = "/v1/models"

// A Model is a System One model offered by the API.
type Model struct {
	// Name is the identifier to pass as [SystemOneRequest.Model].
	Name string `json:"name"`
	// Description says what the model is for.
	Description string `json:"description"`
	// ReleaseDate is the model's release date, as an ISO 8601 date string.
	ReleaseDate string `json:"release_date"`
}

// ModelList is the result of [Client.ListModels].
type ModelList struct {
	// Models are the available models.
	Models []Model `json:"models"`

	// Meta describes the HTTP response the list was decoded from.
	Meta *ResponseMeta `json:"-"`
}

// ListModels returns the models available to the API key.
func (c *Client) ListModels(ctx context.Context, opts ...RequestOption) (*ModelList, error) {
	var list ModelList
	meta, err := c.do(ctx, "GET", modelsPath, nil, &list, opts...)
	if err != nil {
		return nil, err
	}
	list.Meta = meta
	return &list, nil
}
