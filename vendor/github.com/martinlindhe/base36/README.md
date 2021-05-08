# About

[![Travis-CI](https://api.travis-ci.org/martinlindhe/base36.svg)](https://travis-ci.org/martinlindhe/base36)
[![GoDoc](https://godoc.org/github.com/martinlindhe/base36?status.svg)](https://godoc.org/github.com/martinlindhe/base36)

Implements Base36 encoding and decoding, which is useful to represent
large integers in a case-insensitive alphanumeric way.

## Examples

```go
import "github.com/martinlindhe/base36"

fmt.Println(base36.Encode(5481594952936519619))
// Output: 15N9Z8L3AU4EB

fmt.Println(base36.Decode("15N9Z8L3AU4EB"))
// Output: 5481594952936519619

fmt.Println(base36.EncodeBytes([]byte{1, 2, 3, 4}))
// Output: A2F44

fmt.Println(base36.DecodeToBytes("A2F44"))
// Output: [1 2 3 4]
```

## License

Under [MIT](LICENSE)
