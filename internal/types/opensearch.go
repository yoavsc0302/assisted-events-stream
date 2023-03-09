package types

type OpensearchRawConfigDocument struct {
	Source ProjectionConfig `json:"_source"`
	Index  string           `json:"_index"`
	ID     string           `json:"_id"`
}
