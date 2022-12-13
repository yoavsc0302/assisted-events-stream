package projection

type MalformedEventError struct {
	msg string
}

func NewMalformedEventError(msg string) error {
	return &MalformedEventError{
		msg: msg,
	}
}

func (e *MalformedEventError) Error() string {
	return e.msg
}
