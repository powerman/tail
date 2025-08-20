package tail

import "time"

// Defaults for corresponding options.
const (
	DefaultPollDelay   = 200 * time.Millisecond
	DefaultPollTimeout = time.Second
)

// Option let you change Tail behaviour.
type Option interface {
	apply(t *Tail)
}

type optionFunc func(*Tail)

func (f optionFunc) apply(t *Tail) { f(t) }

// PollDelay let you change delay between polling to save CPU.
func PollDelay(d time.Duration) Option {
	return optionFunc(func(t *Tail) { t.pollDelay = d })
}

// PollTimeout let you change how long to wait before return error when
// failed to open or read file.
func PollTimeout(d time.Duration) Option {
	return optionFunc(func(t *Tail) { t.pollTimeout = d })
}

// Whence lets you change where you want to start with tailing the file.
func Whence(w int) Option {
	return optionFunc(func(t *Tail) { t.whence = w })
}
