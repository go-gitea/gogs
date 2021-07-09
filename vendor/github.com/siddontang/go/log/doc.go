// log package supplies more advanced features than go orign log package.
//
// It supports log different level: trace, debug, info, warn, error, fatal.
//
// It also supports different log handlers which you can log to stdout, file, socket, etc...
//
// Use
//
//  import "github.com/siddontang/go/log"
//
//  //log with different level
//  log.Info("hello world")
//  log.Error("hello world")
//
//  //create a logger with specified handler
//  h := NewStreamHandler(os.Stdout)
//  l := log.NewDefault(h)
//  l.Info("hello world")
//  l.Infof("%s %d", "hello", 123)
//
package log
