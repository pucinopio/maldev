//go:build windows

package bof

import (
	"strconv"
)

// expandCFormat is the minimal printf-style expander used by
// BeaconPrintf / BeaconFormatPrintf. It walks the C format string,
// consumes one uintptr per conversion from args (in order), and
// renders each per the CS BOF conventions:
//
//   %d / %i        — int32 (default), int64 with %lld / %I64d
//   %u             — uint32, uint64 with %llu / %I64u
//   %x / %X        — hex 32-bit, 64-bit with %llx / %I64x
//   %p             — pointer (hex, 0x prefix)
//   %s             — char * (NUL-terminated)
//   %c             — single byte
//   %%             — literal percent
//   %lN            — Windows "l" length modifier (single l = 32 bit;
//                    double "ll" = 64 bit)
//   %I64x          — Microsoft 64-bit length modifier
//
// Unknown verbs are emitted literally with the percent sign so the
// operator can spot mismatched format strings instead of getting
// silent garbage.
//
// Excess args are ignored; missing args read as zero.
//
// Why not fmt.Sprintf? Because the input is a C format string with
// raw uintptr varargs from a syscall.NewCallback thunk — fmt.Sprintf
// expects Go format verbs (%v / %d / %s with Go-typed args) and
// would need a separate conversion layer that loses information at
// the boundary.
func expandCFormat(format string, args []uintptr) []byte {
	var b []byte
	idx := 0
	next := func() uintptr {
		if idx < len(args) {
			v := args[idx]
			idx++
			return v
		}
		return 0
	}

	pos := 0
	for pos < len(format) {
		c := format[pos]
		if c != '%' {
			b = append(b, c)
			pos++
			continue
		}
		pos++
		if pos >= len(format) {
			break
		}
		// Length modifiers
		long := false
		switch {
		case format[pos] == 'l':
			pos++
			if pos < len(format) && format[pos] == 'l' {
				long = true
				pos++
			}
			// single 'l' on Windows is 32-bit; long flag stays false
		case format[pos] == 'I' && pos+2 < len(format) && format[pos+1] == '6' && format[pos+2] == '4':
			long = true
			pos += 3
		}
		if pos >= len(format) {
			// Trailing length modifier with no verb — drop it
			break
		}
		verb := format[pos]
		pos++

		switch verb {
		case '%':
			b = append(b, '%')
		case 'd', 'i':
			v := next()
			if long {
				b = strconv.AppendInt(b, int64(v), 10)
			} else {
				b = strconv.AppendInt(b, int64(int32(v)), 10)
			}
		case 'u':
			v := next()
			if long {
				b = strconv.AppendUint(b, uint64(v), 10)
			} else {
				b = strconv.AppendUint(b, uint64(uint32(v)), 10)
			}
		case 'x':
			v := next()
			if long {
				b = strconv.AppendUint(b, uint64(v), 16)
			} else {
				b = strconv.AppendUint(b, uint64(uint32(v)), 16)
			}
		case 'X':
			v := next()
			start := len(b)
			if long {
				b = strconv.AppendUint(b, uint64(v), 16)
			} else {
				b = strconv.AppendUint(b, uint64(uint32(v)), 16)
			}
			upperASCII(b[start:])
		case 'p':
			v := next()
			b = append(b, '0', 'x')
			b = strconv.AppendUint(b, uint64(v), 16)
		case 's':
			ptr := next()
			if ptr != 0 {
				b = append(b, cStringFromPtr(ptr, 65535)...)
			} else {
				b = append(b, "(null)"...)
			}
		case 'c':
			b = append(b, byte(next()))
		default:
			// Unknown verb — emit `%<verb>` literally without consuming
			// an arg, so subsequent valid conversions stay aligned with
			// the operator's intent.
			b = append(b, '%', verb)
		}
	}
	return b
}

// upperASCII upper-cases ASCII hex digits in place (a..f → A..F).
func upperASCII(b []byte) {
	for i, c := range b {
		if c >= 'a' && c <= 'f' {
			b[i] = c - 32
		}
	}
}
