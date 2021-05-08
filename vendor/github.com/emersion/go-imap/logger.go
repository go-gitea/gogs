package imap

// Logger is the behaviour used by server/client to
// report errors for accepting connections and unexpected behavior from handlers.
type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}
