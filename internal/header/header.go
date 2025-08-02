package header

// ValidHeaderField checks if the header field is valid.
func ValidHeaderField(field string) bool {
	fieldBytes := []byte(field)
	for _, c := range fieldBytes {
		if !validHeaderFieldByte(c) {
			return false
		}
	}

	return true
}

// ValidHeaderValue checks if the header value is valid.
func ValidHeaderValue(value string) bool {
	valueBytes := []byte(value)
	for _, c := range valueBytes {
		if !validHeaderValueByte(c) {
			return false
		}
	}

	return true
}

// validHeaderFieldByte reports whether c is a valid byte in a header
// field name. RFC 7230 says:
//
//	header-field   = field-name ":" OWS field-value OWS
//	field-name     = token
//	tchar = "!" / "#" / "$" / "%" / "&" / "'" / "*" / "+" / "-" / "." /
//	        "^" / "_" / "`" / "|" / "~" / DIGIT / ALPHA
//	token = 1*tchar
//
// From https://github.com/golang/go/blob/3d33437c450aa74014ea1d41cd986b6ee6266984/src/net/textproto/reader.go#L678
func validHeaderFieldByte(c byte) bool {
	// mask is a 128-bit bitmap with 1s for allowed bytes,
	// so that the byte c can be tested with a shift and an and.
	// If c >= 128, then 1<<c and 1<<(c-64) will both be zero,
	// and this function will return false.
	const mask = 0 |
		(1<<(10)-1)<<'0' |
		(1<<(26)-1)<<'a' |
		(1<<(26)-1)<<'A' |
		1<<'!' |
		1<<'#' |
		1<<'$' |
		1<<'%' |
		1<<'&' |
		1<<'\'' |
		1<<'*' |
		1<<'+' |
		1<<'-' |
		1<<'.' |
		1<<'^' |
		1<<'_' |
		1<<'`' |
		1<<'|' |
		1<<'~'

	return ((uint64(1)<<c)&(mask&(1<<64-1)) |
		(uint64(1)<<(c-64))&(mask>>64)) != 0
}

// validHeaderValueByte reports whether c is a valid byte in a header
// field value. RFC 7230 says:
//
//	field-content  = field-vchar [ 1*( SP / HTAB ) field-vchar ]
//	field-vchar    = VCHAR / obs-text
//	obs-text       = %x80-FF
//
// RFC 5234 says:
//
//	HTAB           =  %x09
//	SP             =  %x20
//	VCHAR          =  %x21-7E
//
// From https://github.com/golang/go/blob/3d33437c450aa74014ea1d41cd986b6ee6266984/src/net/textproto/reader.go#L718
func validHeaderValueByte(c byte) bool {
	// mask is a 128-bit bitmap with 1s for allowed bytes,
	// so that the byte c can be tested with a shift and an and.
	// If c >= 128, then 1<<c and 1<<(c-64) will both be zero.
	// Since this is the obs-text range, we invert the mask to
	// create a bitmap with 1s for disallowed bytes.
	const mask = 0 |
		(1<<(0x7f-0x21)-1)<<0x21 | // VCHAR: %x21-7E
		1<<0x20 | // SP: %x20
		1<<0x09 // HTAB: %x09

	return ((uint64(1)<<c)&^(mask&(1<<64-1)) |
		(uint64(1)<<(c-64))&^(mask>>64)) == 0
}
