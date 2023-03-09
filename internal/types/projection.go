package types

const (
	ProjectionModeOnline  = "online"
	ProjectionModeOffline = "offline"
)

type ProjectionConfig struct {
	Mode ProjectionMode
}

type ProjectionMode string
