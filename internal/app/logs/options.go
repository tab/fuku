package logs

// Options sizes the replay history and the live buffer of each subscription
type Options struct {
	Buffer  int
	History int
}
