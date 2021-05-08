package mail

import (
	"mime"
	"net/mail"
	"strings"

	"github.com/emersion/go-message"
)

// Address represents a single mail address.
type Address mail.Address

// String formats the address as a valid RFC 5322 address. If the address's name
// contains non-ASCII characters the name will be rendered according to
// RFC 2047.
//
// Don't use this function to set a message header field, instead use
// Header.SetAddressList.
func (a *Address) String() string {
	return ((*mail.Address)(a)).String()
}

func parseAddressList(s string) ([]*Address, error) {
	parser := mail.AddressParser{
		&mime.WordDecoder{message.CharsetReader},
	}
	list, err := parser.ParseList(s)
	if err != nil {
		return nil, err
	}

	addrs := make([]*Address, len(list))
	for i, a := range list {
		addrs[i] = (*Address)(a)
	}
	return addrs, nil
}

func formatAddressList(l []*Address) string {
	formatted := make([]string, len(l))
	for i, a := range l {
		formatted[i] = a.String()
	}
	return strings.Join(formatted, ", ")
}
