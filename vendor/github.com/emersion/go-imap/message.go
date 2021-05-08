package imap

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"strconv"
	"strings"
	"time"
)

// System message flags, defined in RFC 3501 section 2.3.2.
const (
	SeenFlag     = "\\Seen"
	AnsweredFlag = "\\Answered"
	FlaggedFlag  = "\\Flagged"
	DeletedFlag  = "\\Deleted"
	DraftFlag    = "\\Draft"
	RecentFlag   = "\\Recent"
)

// TryCreateFlag is a special flag in MailboxStatus.PermanentFlags indicating
// that it is possible to create new keywords by attempting to store those
// flags in the mailbox.
const TryCreateFlag = "\\*"

var flags = []string{
	SeenFlag,
	AnsweredFlag,
	FlaggedFlag,
	DeletedFlag,
	DraftFlag,
	RecentFlag,
}

// A PartSpecifier specifies which parts of the MIME entity should be returned.
type PartSpecifier string

// Part specifiers described in RFC 3501 page 55.
const (
	// Refers to the entire part, including headers.
	EntireSpecifier PartSpecifier = ""
	// Refers to the header of the part. Must include the final CRLF delimiting
	// the header and the body.
	HeaderSpecifier = "HEADER"
	// Refers to the text body of the part, omitting the header.
	TextSpecifier = "TEXT"
	// Refers to the MIME Internet Message Body header.  Must include the final
	// CRLF delimiting the header and the body.
	MIMESpecifier = "MIME"
)

// CanonicalFlag returns the canonical form of a flag. Flags are case-insensitive.
//
// If the flag is defined in RFC 3501, it returns the flag with the case of the
// RFC. Otherwise, it returns the lowercase version of the flag.
func CanonicalFlag(flag string) string {
	flag = strings.ToLower(flag)
	for _, f := range flags {
		if strings.ToLower(f) == flag {
			return f
		}
	}
	return flag
}

func ParseParamList(fields []interface{}) (map[string]string, error) {
	params := make(map[string]string)

	var k string
	for i, f := range fields {
		p, err := ParseString(f)
		if err != nil {
			return nil, errors.New("Parameter list contains a non-string: " + err.Error())
		}

		if i%2 == 0 {
			k = p
		} else {
			params[k] = p
			k = ""
		}
	}

	if k != "" {
		return nil, errors.New("Parameter list contains a key without a value")
	}
	return params, nil
}

func FormatParamList(params map[string]string) []interface{} {
	var fields []interface{}
	for key, value := range params {
		fields = append(fields, key, value)
	}
	return fields
}

var wordDecoder = &mime.WordDecoder{
	CharsetReader: func(charset string, input io.Reader) (io.Reader, error) {
		if CharsetReader != nil {
			return CharsetReader(charset, input)
		}
		return nil, fmt.Errorf("imap: unhandled charset %q", charset)
	},
}

func decodeHeader(s string) (string, error) {
	dec, err := wordDecoder.DecodeHeader(s)
	if err != nil {
		return s, err
	}
	return dec, nil
}

func encodeHeader(s string) string {
	return mime.QEncoding.Encode("utf-8", s)
}

func stringLowered(i interface{}) (string, bool) {
	s, ok := i.(string)
	return strings.ToLower(s), ok
}

func parseHeaderParamList(fields []interface{}) (map[string]string, error) {
	params, err := ParseParamList(fields)
	if err != nil {
		return nil, err
	}

	for k, v := range params {
		if lower := strings.ToLower(k); lower != k {
			delete(params, k)
			k = lower
		}

		params[k], _ = decodeHeader(v)
	}

	return params, nil
}

func formatHeaderParamList(params map[string]string) []interface{} {
	encoded := make(map[string]string)
	for k, v := range params {
		encoded[k] = encodeHeader(v)
	}
	return FormatParamList(encoded)
}

// A message.
type Message struct {
	// The message sequence number. It must be greater than or equal to 1.
	SeqNum uint32
	// The mailbox items that are currently filled in. This map's values
	// should not be used directly, they must only be used by libraries
	// implementing extensions of the IMAP protocol.
	Items map[FetchItem]interface{}

	// The message envelope.
	Envelope *Envelope
	// The message body structure (either BODYSTRUCTURE or BODY).
	BodyStructure *BodyStructure
	// The message flags.
	Flags []string
	// The date the message was received by the server.
	InternalDate time.Time
	// The message size.
	Size uint32
	// The message unique identifier. It must be greater than or equal to 1.
	Uid uint32
	// The message body sections.
	Body map[*BodySectionName]Literal

	// The order in which items were requested. This order must be preserved
	// because some bad IMAP clients (looking at you, Outlook!) refuse responses
	// containing items in a different order.
	itemsOrder []FetchItem
}

// Create a new empty message that will contain the specified items.
func NewMessage(seqNum uint32, items []FetchItem) *Message {
	msg := &Message{
		SeqNum:     seqNum,
		Items:      make(map[FetchItem]interface{}),
		Body:       make(map[*BodySectionName]Literal),
		itemsOrder: items,
	}

	for _, k := range items {
		msg.Items[k] = nil
	}

	return msg
}

// Parse a message from fields.
func (m *Message) Parse(fields []interface{}) error {
	m.Items = make(map[FetchItem]interface{})
	m.Body = map[*BodySectionName]Literal{}
	m.itemsOrder = nil

	var k FetchItem
	for i, f := range fields {
		if i%2 == 0 { // It's a key
			switch f := f.(type) {
			case string:
				k = FetchItem(strings.ToUpper(f))
			case RawString:
				k = FetchItem(strings.ToUpper(string(f)))
			default:
				return fmt.Errorf("cannot parse message: key is not a string, but a %T", f)
			}
		} else { // It's a value
			m.Items[k] = nil
			m.itemsOrder = append(m.itemsOrder, k)

			switch k {
			case FetchBody, FetchBodyStructure:
				bs, ok := f.([]interface{})
				if !ok {
					return fmt.Errorf("cannot parse message: BODYSTRUCTURE is not a list, but a %T", f)
				}

				m.BodyStructure = &BodyStructure{Extended: k == FetchBodyStructure}
				if err := m.BodyStructure.Parse(bs); err != nil {
					return err
				}
			case FetchEnvelope:
				env, ok := f.([]interface{})
				if !ok {
					return fmt.Errorf("cannot parse message: ENVELOPE is not a list, but a %T", f)
				}

				m.Envelope = &Envelope{}
				if err := m.Envelope.Parse(env); err != nil {
					return err
				}
			case FetchFlags:
				flags, ok := f.([]interface{})
				if !ok {
					return fmt.Errorf("cannot parse message: FLAGS is not a list, but a %T", f)
				}

				m.Flags = make([]string, len(flags))
				for i, flag := range flags {
					s, _ := ParseString(flag)
					m.Flags[i] = CanonicalFlag(s)
				}
			case FetchInternalDate:
				date, _ := f.(string)
				m.InternalDate, _ = time.Parse(DateTimeLayout, date)
			case FetchRFC822Size:
				m.Size, _ = ParseNumber(f)
			case FetchUid:
				m.Uid, _ = ParseNumber(f)
			default:
				// Likely to be a section of the body
				// First check that the section name is correct
				if section, err := ParseBodySectionName(k); err != nil {
					// Not a section name, maybe an attribute defined in an IMAP extension
					m.Items[k] = f
				} else {
					m.Body[section], _ = f.(Literal)
				}
			}
		}
	}

	return nil
}

func (m *Message) formatItem(k FetchItem) []interface{} {
	v := m.Items[k]
	var kk interface{} = RawString(k)

	switch k {
	case FetchBody, FetchBodyStructure:
		// Extension data is only returned with the BODYSTRUCTURE fetch
		m.BodyStructure.Extended = k == FetchBodyStructure
		v = m.BodyStructure.Format()
	case FetchEnvelope:
		v = m.Envelope.Format()
	case FetchFlags:
		flags := make([]interface{}, len(m.Flags))
		for i, flag := range m.Flags {
			flags[i] = RawString(flag)
		}
		v = flags
	case FetchInternalDate:
		v = m.InternalDate
	case FetchRFC822Size:
		v = m.Size
	case FetchUid:
		v = m.Uid
	default:
		for section, literal := range m.Body {
			if section.value == k {
				// This can contain spaces, so we can't pass it as a string directly
				kk = section.resp()
				v = literal
				break
			}
		}
	}

	return []interface{}{kk, v}
}

func (m *Message) Format() []interface{} {
	var fields []interface{}

	// First send ordered items
	processed := make(map[FetchItem]bool)
	for _, k := range m.itemsOrder {
		if _, ok := m.Items[k]; ok {
			fields = append(fields, m.formatItem(k)...)
			processed[k] = true
		}
	}

	// Then send other remaining items
	for k := range m.Items {
		if !processed[k] {
			fields = append(fields, m.formatItem(k)...)
		}
	}

	return fields
}

// GetBody gets the body section with the specified name. Returns nil if it's not found.
func (m *Message) GetBody(section *BodySectionName) Literal {
	section = section.resp()

	for s, body := range m.Body {
		if section.Equal(s) {
			if body == nil {
				// Server can return nil, we need to treat as empty string per RFC 3501
				body = bytes.NewReader(nil)
			}
			return body
		}
	}
	return nil
}

// A body section name.
// See RFC 3501 page 55.
type BodySectionName struct {
	BodyPartName

	// If set to true, do not implicitly set the \Seen flag.
	Peek bool
	// The substring of the section requested. The first value is the position of
	// the first desired octet and the second value is the maximum number of
	// octets desired.
	Partial []int

	value FetchItem
}

func (section *BodySectionName) parse(s string) error {
	section.value = FetchItem(s)

	if s == "RFC822" {
		s = "BODY[]"
	}
	if s == "RFC822.HEADER" {
		s = "BODY.PEEK[HEADER]"
	}
	if s == "RFC822.TEXT" {
		s = "BODY[TEXT]"
	}

	partStart := strings.Index(s, "[")
	if partStart == -1 {
		return errors.New("Invalid body section name: must contain an open bracket")
	}

	partEnd := strings.LastIndex(s, "]")
	if partEnd == -1 {
		return errors.New("Invalid body section name: must contain a close bracket")
	}

	name := s[:partStart]
	part := s[partStart+1 : partEnd]
	partial := s[partEnd+1:]

	if name == "BODY.PEEK" {
		section.Peek = true
	} else if name != "BODY" {
		return errors.New("Invalid body section name")
	}

	b := bytes.NewBufferString(part + string(cr) + string(lf))
	r := NewReader(b)
	fields, err := r.ReadFields()
	if err != nil {
		return err
	}

	if err := section.BodyPartName.parse(fields); err != nil {
		return err
	}

	if len(partial) > 0 {
		if !strings.HasPrefix(partial, "<") || !strings.HasSuffix(partial, ">") {
			return errors.New("Invalid body section name: invalid partial")
		}
		partial = partial[1 : len(partial)-1]

		partialParts := strings.SplitN(partial, ".", 2)

		var from, length int
		if from, err = strconv.Atoi(partialParts[0]); err != nil {
			return errors.New("Invalid body section name: invalid partial: invalid from: " + err.Error())
		}
		section.Partial = []int{from}

		if len(partialParts) == 2 {
			if length, err = strconv.Atoi(partialParts[1]); err != nil {
				return errors.New("Invalid body section name: invalid partial: invalid length: " + err.Error())
			}
			section.Partial = append(section.Partial, length)
		}
	}

	return nil
}

func (section *BodySectionName) FetchItem() FetchItem {
	if section.value != "" {
		return section.value
	}

	s := "BODY"
	if section.Peek {
		s += ".PEEK"
	}

	s += "[" + section.BodyPartName.string() + "]"

	if len(section.Partial) > 0 {
		s += "<"
		s += strconv.Itoa(section.Partial[0])

		if len(section.Partial) > 1 {
			s += "."
			s += strconv.Itoa(section.Partial[1])
		}

		s += ">"
	}

	return FetchItem(s)
}

// Equal checks whether two sections are equal.
func (section *BodySectionName) Equal(other *BodySectionName) bool {
	if section.Peek != other.Peek {
		return false
	}
	if len(section.Partial) != len(other.Partial) {
		return false
	}
	if len(section.Partial) > 0 && section.Partial[0] != other.Partial[0] {
		return false
	}
	if len(section.Partial) > 1 && section.Partial[1] != other.Partial[1] {
		return false
	}
	return section.BodyPartName.Equal(&other.BodyPartName)
}

func (section *BodySectionName) resp() *BodySectionName {
	resp := *section // Copy section
	if resp.Peek {
		resp.Peek = false
	}
	if len(resp.Partial) == 2 {
		resp.Partial = []int{resp.Partial[0]}
	}
	if !strings.HasPrefix(string(resp.value), string(FetchRFC822)) {
		resp.value = ""
	}
	return &resp
}

// ExtractPartial returns a subset of the specified bytes matching the partial requested in the
// section name.
func (section *BodySectionName) ExtractPartial(b []byte) []byte {
	if len(section.Partial) != 2 {
		return b
	}

	from := section.Partial[0]
	length := section.Partial[1]
	to := from + length
	if from > len(b) {
		return nil
	}
	if to > len(b) {
		to = len(b)
	}
	return b[from:to]
}

// ParseBodySectionName parses a body section name.
func ParseBodySectionName(s FetchItem) (*BodySectionName, error) {
	section := new(BodySectionName)
	err := section.parse(string(s))
	return section, err
}

// A body part name.
type BodyPartName struct {
	// The specifier of the requested part.
	Specifier PartSpecifier
	// The part path. Parts indexes start at 1.
	Path []int
	// If Specifier is HEADER, contains header fields that will/won't be returned,
	// depending of the value of NotFields.
	Fields []string
	// If set to true, Fields is a blacklist of fields instead of a whitelist.
	NotFields bool
}

func (part *BodyPartName) parse(fields []interface{}) error {
	if len(fields) == 0 {
		return nil
	}

	name, ok := fields[0].(string)
	if !ok {
		return errors.New("Invalid body section name: part name must be a string")
	}

	args := fields[1:]

	path := strings.Split(strings.ToUpper(name), ".")

	end := 0
loop:
	for i, node := range path {
		switch PartSpecifier(node) {
		case EntireSpecifier, HeaderSpecifier, MIMESpecifier, TextSpecifier:
			part.Specifier = PartSpecifier(node)
			end = i + 1
			break loop
		}

		index, err := strconv.Atoi(node)
		if err != nil {
			return errors.New("Invalid body part name: " + err.Error())
		}
		if index <= 0 {
			return errors.New("Invalid body part name: index <= 0")
		}

		part.Path = append(part.Path, index)
	}

	if part.Specifier == HeaderSpecifier && len(path) > end && path[end] == "FIELDS" && len(args) > 0 {
		end++
		if len(path) > end && path[end] == "NOT" {
			part.NotFields = true
		}

		names, ok := args[0].([]interface{})
		if !ok {
			return errors.New("Invalid body part name: HEADER.FIELDS must have a list argument")
		}

		for _, namei := range names {
			if name, ok := namei.(string); ok {
				part.Fields = append(part.Fields, name)
			}
		}
	}

	return nil
}

func (part *BodyPartName) string() string {
	path := make([]string, len(part.Path))
	for i, index := range part.Path {
		path[i] = strconv.Itoa(index)
	}

	if part.Specifier != EntireSpecifier {
		path = append(path, string(part.Specifier))
	}

	if part.Specifier == HeaderSpecifier && len(part.Fields) > 0 {
		path = append(path, "FIELDS")

		if part.NotFields {
			path = append(path, "NOT")
		}
	}

	s := strings.Join(path, ".")

	if len(part.Fields) > 0 {
		s += " (" + strings.Join(part.Fields, " ") + ")"
	}

	return s
}

// Equal checks whether two body part names are equal.
func (part *BodyPartName) Equal(other *BodyPartName) bool {
	if part.Specifier != other.Specifier {
		return false
	}
	if part.NotFields != other.NotFields {
		return false
	}
	if len(part.Path) != len(other.Path) {
		return false
	}
	for i, node := range part.Path {
		if node != other.Path[i] {
			return false
		}
	}
	if len(part.Fields) != len(other.Fields) {
		return false
	}
	for _, field := range part.Fields {
		found := false
		for _, f := range other.Fields {
			if strings.EqualFold(field, f) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// An address.
type Address struct {
	// The personal name.
	PersonalName string
	// The SMTP at-domain-list (source route).
	AtDomainList string
	// The mailbox name.
	MailboxName string
	// The host name.
	HostName string
}

// Address returns the mailbox address (e.g. "foo@example.org").
func (addr *Address) Address() string {
	return addr.MailboxName + "@" + addr.HostName
}

// Parse an address from fields.
func (addr *Address) Parse(fields []interface{}) error {
	if len(fields) < 4 {
		return errors.New("Address doesn't contain 4 fields")
	}

	if s, err := ParseString(fields[0]); err == nil {
		addr.PersonalName, _ = decodeHeader(s)
	}
	if s, err := ParseString(fields[1]); err == nil {
		addr.AtDomainList, _ = decodeHeader(s)
	}

	s, err := ParseString(fields[2])
	if err != nil {
		return errors.New("Mailbox name could not be parsed")
	}
	addr.MailboxName, _ = decodeHeader(s)

	s, err = ParseString(fields[3])
	if err != nil {
		return errors.New("Host name could not be parsed")
	}
	addr.HostName, _ = decodeHeader(s)

	return nil
}

// Format an address to fields.
func (addr *Address) Format() []interface{} {
	fields := make([]interface{}, 4)

	if addr.PersonalName != "" {
		fields[0] = encodeHeader(addr.PersonalName)
	}
	if addr.AtDomainList != "" {
		fields[1] = addr.AtDomainList
	}
	if addr.MailboxName != "" {
		fields[2] = addr.MailboxName
	}
	if addr.HostName != "" {
		fields[3] = addr.HostName
	}

	return fields
}

// Parse an address list from fields.
func ParseAddressList(fields []interface{}) (addrs []*Address) {
	for _, f := range fields {
		if addrFields, ok := f.([]interface{}); ok {
			addr := &Address{}
			if err := addr.Parse(addrFields); err == nil {
				addrs = append(addrs, addr)
			}
		}
	}

	return
}

// Format an address list to fields.
func FormatAddressList(addrs []*Address) interface{} {
	if len(addrs) == 0 {
		return nil
	}

	fields := make([]interface{}, len(addrs))

	for i, addr := range addrs {
		fields[i] = addr.Format()
	}

	return fields
}

// A message envelope, ie. message metadata from its headers.
// See RFC 3501 page 77.
type Envelope struct {
	// The message date.
	Date time.Time
	// The message subject.
	Subject string
	// The From header addresses.
	From []*Address
	// The message senders.
	Sender []*Address
	// The Reply-To header addresses.
	ReplyTo []*Address
	// The To header addresses.
	To []*Address
	// The Cc header addresses.
	Cc []*Address
	// The Bcc header addresses.
	Bcc []*Address
	// The In-Reply-To header. Contains the parent Message-Id.
	InReplyTo string
	// The Message-Id header.
	MessageId string
}

// Parse an envelope from fields.
func (e *Envelope) Parse(fields []interface{}) error {
	if len(fields) < 10 {
		return errors.New("ENVELOPE doesn't contain 10 fields")
	}

	if date, ok := fields[0].(string); ok {
		e.Date, _ = parseMessageDateTime(date)
	}
	if subject, err := ParseString(fields[1]); err == nil {
		e.Subject, _ = decodeHeader(subject)
	}
	if from, ok := fields[2].([]interface{}); ok {
		e.From = ParseAddressList(from)
	}
	if sender, ok := fields[3].([]interface{}); ok {
		e.Sender = ParseAddressList(sender)
	}
	if replyTo, ok := fields[4].([]interface{}); ok {
		e.ReplyTo = ParseAddressList(replyTo)
	}
	if to, ok := fields[5].([]interface{}); ok {
		e.To = ParseAddressList(to)
	}
	if cc, ok := fields[6].([]interface{}); ok {
		e.Cc = ParseAddressList(cc)
	}
	if bcc, ok := fields[7].([]interface{}); ok {
		e.Bcc = ParseAddressList(bcc)
	}
	if inReplyTo, ok := fields[8].(string); ok {
		e.InReplyTo = inReplyTo
	}
	if msgId, ok := fields[9].(string); ok {
		e.MessageId = msgId
	}

	return nil
}

// Format an envelope to fields.
func (e *Envelope) Format() (fields []interface{}) {
	fields = make([]interface{}, 0, 10)
	fields = append(fields, envelopeDateTime(e.Date))
	if e.Subject != "" {
		fields = append(fields, encodeHeader(e.Subject))
	} else {
		fields = append(fields, nil)
	}
	fields = append(fields,
		FormatAddressList(e.From),
		FormatAddressList(e.Sender),
		FormatAddressList(e.ReplyTo),
		FormatAddressList(e.To),
		FormatAddressList(e.Cc),
		FormatAddressList(e.Bcc),
	)
	if e.InReplyTo != "" {
		fields = append(fields, e.InReplyTo)
	} else {
		fields = append(fields, nil)
	}
	if e.MessageId != "" {
		fields = append(fields, e.MessageId)
	} else {
		fields = append(fields, nil)
	}
	return fields
}

// A body structure.
// See RFC 3501 page 74.
type BodyStructure struct {
	// Basic fields

	// The MIME type (e.g. "text", "image")
	MIMEType string
	// The MIME subtype (e.g. "plain", "png")
	MIMESubType string
	// The MIME parameters.
	Params map[string]string

	// The Content-Id header.
	Id string
	// The Content-Description header.
	Description string
	// The Content-Encoding header.
	Encoding string
	// The Content-Length header.
	Size uint32

	// Type-specific fields

	// The children parts, if multipart.
	Parts []*BodyStructure
	// The envelope, if message/rfc822.
	Envelope *Envelope
	// The body structure, if message/rfc822.
	BodyStructure *BodyStructure
	// The number of lines, if text or message/rfc822.
	Lines uint32

	// Extension data

	// True if the body structure contains extension data.
	Extended bool

	// The Content-Disposition header field value.
	Disposition string
	// The Content-Disposition header field parameters.
	DispositionParams map[string]string
	// The Content-Language header field, if multipart.
	Language []string
	// The content URI, if multipart.
	Location []string

	// The MD5 checksum.
	MD5 string
}

func (bs *BodyStructure) Parse(fields []interface{}) error {
	if len(fields) == 0 {
		return nil
	}

	// Initialize params map
	bs.Params = make(map[string]string)

	switch fields[0].(type) {
	case []interface{}: // A multipart body part
		bs.MIMEType = "multipart"

		end := 0
		for i, fi := range fields {
			switch f := fi.(type) {
			case []interface{}: // A part
				part := new(BodyStructure)
				if err := part.Parse(f); err != nil {
					return err
				}
				bs.Parts = append(bs.Parts, part)
			case string:
				end = i
			}

			if end > 0 {
				break
			}
		}

		bs.MIMESubType, _ = fields[end].(string)
		end++

		// GMail seems to return only 3 extension data fields. Parse as many fields
		// as we can.
		if len(fields) > end {
			bs.Extended = true // Contains extension data

			params, _ := fields[end].([]interface{})
			bs.Params, _ = parseHeaderParamList(params)
			end++
		}
		if len(fields) > end {
			if disp, ok := fields[end].([]interface{}); ok && len(disp) >= 2 {
				if s, ok := disp[0].(string); ok {
					bs.Disposition, _ = decodeHeader(s)
					bs.Disposition = strings.ToLower(bs.Disposition)
				}
				if params, ok := disp[1].([]interface{}); ok {
					bs.DispositionParams, _ = parseHeaderParamList(params)
				}
			}
			end++
		}
		if len(fields) > end {
			switch langs := fields[end].(type) {
			case string:
				bs.Language = []string{langs}
			case []interface{}:
				bs.Language, _ = ParseStringList(langs)
			default:
				bs.Language = nil
			}
			end++
		}
		if len(fields) > end {
			location, _ := fields[end].([]interface{})
			bs.Location, _ = ParseStringList(location)
			end++
		}
	case string: // A non-multipart body part
		if len(fields) < 7 {
			return errors.New("Non-multipart body part doesn't have 7 fields")
		}

		bs.MIMEType, _ = stringLowered(fields[0])
		bs.MIMESubType, _ = stringLowered(fields[1])

		params, _ := fields[2].([]interface{})
		bs.Params, _ = parseHeaderParamList(params)

		bs.Id, _ = fields[3].(string)
		if desc, err := ParseString(fields[4]); err == nil {
			bs.Description, _ = decodeHeader(desc)
		}
		bs.Encoding, _ = stringLowered(fields[5])
		bs.Size, _ = ParseNumber(fields[6])

		end := 7

		// Type-specific fields
		if strings.EqualFold(bs.MIMEType, "message") && strings.EqualFold(bs.MIMESubType, "rfc822") {
			if len(fields)-end < 3 {
				return errors.New("Missing type-specific fields for message/rfc822")
			}

			envelope, _ := fields[end].([]interface{})
			bs.Envelope = new(Envelope)
			bs.Envelope.Parse(envelope)

			structure, _ := fields[end+1].([]interface{})
			bs.BodyStructure = new(BodyStructure)
			bs.BodyStructure.Parse(structure)

			bs.Lines, _ = ParseNumber(fields[end+2])

			end += 3
		}
		if strings.EqualFold(bs.MIMEType, "text") {
			if len(fields)-end < 1 {
				return errors.New("Missing type-specific fields for text/*")
			}

			bs.Lines, _ = ParseNumber(fields[end])
			end++
		}

		// GMail seems to return only 3 extension data fields. Parse as many fields
		// as we can.
		if len(fields) > end {
			bs.Extended = true // Contains extension data

			bs.MD5, _ = fields[end].(string)
			end++
		}
		if len(fields) > end {
			if disp, ok := fields[end].([]interface{}); ok && len(disp) >= 2 {
				if s, ok := disp[0].(string); ok {
					bs.Disposition, _ = decodeHeader(s)
					bs.Disposition = strings.ToLower(bs.Disposition)
				}
				if params, ok := disp[1].([]interface{}); ok {
					bs.DispositionParams, _ = parseHeaderParamList(params)
				}
			}
			end++
		}
		if len(fields) > end {
			switch langs := fields[end].(type) {
			case string:
				bs.Language = []string{langs}
			case []interface{}:
				bs.Language, _ = ParseStringList(langs)
			default:
				bs.Language = nil
			}
			end++
		}
		if len(fields) > end {
			location, _ := fields[end].([]interface{})
			bs.Location, _ = ParseStringList(location)
			end++
		}
	}

	return nil
}

func (bs *BodyStructure) Format() (fields []interface{}) {
	if strings.EqualFold(bs.MIMEType, "multipart") {
		for _, part := range bs.Parts {
			fields = append(fields, part.Format())
		}

		fields = append(fields, bs.MIMESubType)

		if bs.Extended {
			extended := make([]interface{}, 4)

			if bs.Params != nil {
				extended[0] = formatHeaderParamList(bs.Params)
			}
			if bs.Disposition != "" {
				extended[1] = []interface{}{
					encodeHeader(bs.Disposition),
					formatHeaderParamList(bs.DispositionParams),
				}
			}
			if bs.Language != nil {
				extended[2] = FormatStringList(bs.Language)
			}
			if bs.Location != nil {
				extended[3] = FormatStringList(bs.Location)
			}

			fields = append(fields, extended...)
		}
	} else {
		fields = make([]interface{}, 7)
		fields[0] = bs.MIMEType
		fields[1] = bs.MIMESubType
		fields[2] = formatHeaderParamList(bs.Params)

		if bs.Id != "" {
			fields[3] = bs.Id
		}
		if bs.Description != "" {
			fields[4] = encodeHeader(bs.Description)
		}
		if bs.Encoding != "" {
			fields[5] = bs.Encoding
		}

		fields[6] = bs.Size

		// Type-specific fields
		if strings.EqualFold(bs.MIMEType, "message") && strings.EqualFold(bs.MIMESubType, "rfc822") {
			var env interface{}
			if bs.Envelope != nil {
				env = bs.Envelope.Format()
			}

			var bsbs interface{}
			if bs.BodyStructure != nil {
				bsbs = bs.BodyStructure.Format()
			}

			fields = append(fields, env, bsbs, bs.Lines)
		}
		if strings.EqualFold(bs.MIMEType, "text") {
			fields = append(fields, bs.Lines)
		}

		// Extension data
		if bs.Extended {
			extended := make([]interface{}, 4)

			if bs.MD5 != "" {
				extended[0] = bs.MD5
			}
			if bs.Disposition != "" {
				extended[1] = []interface{}{
					encodeHeader(bs.Disposition),
					formatHeaderParamList(bs.DispositionParams),
				}
			}
			if bs.Language != nil {
				extended[2] = FormatStringList(bs.Language)
			}
			if bs.Location != nil {
				extended[3] = FormatStringList(bs.Location)
			}

			fields = append(fields, extended...)
		}
	}

	return
}

// Filename parses the body structure's filename, if it's an attachment. An
// empty string is returned if the filename isn't specified. An error is
// returned if and only if a charset error occurs, in which case the undecoded
// filename is returned too.
func (bs *BodyStructure) Filename() (string, error) {
	raw, ok := bs.DispositionParams["filename"]
	if !ok {
		// Using "name" in Content-Type is discouraged
		raw = bs.Params["name"]
	}
	return decodeHeader(raw)
}

// BodyStructureWalkFunc is the type of the function called for each body
// structure visited by BodyStructure.Walk. The path argument contains the IMAP
// part path (see BodyPartName).
//
// The function should return true to visit all of the part's children or false
// to skip them.
type BodyStructureWalkFunc func(path []int, part *BodyStructure) (walkChildren bool)

// Walk walks the body structure tree, calling f for each part in the tree,
// including bs itself. The parts are visited in DFS pre-order.
func (bs *BodyStructure) Walk(f BodyStructureWalkFunc) {
	// Non-multipart messages only have part 1
	if len(bs.Parts) == 0 {
		f([]int{1}, bs)
		return
	}

	bs.walk(f, nil)
}

func (bs *BodyStructure) walk(f BodyStructureWalkFunc, path []int) {
	if !f(path, bs) {
		return
	}

	for i, part := range bs.Parts {
		num := i + 1

		partPath := append([]int(nil), path...)
		partPath = append(partPath, num)

		part.walk(f, partPath)
	}
}
