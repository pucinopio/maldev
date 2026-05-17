package stage1

import (
	"fmt"

	"github.com/oioio-space/maldev/pe/packer/stubgen/amd64"
)

// emitLZ4DecompressBlock writes the LZ4 register-setup + inflate +
// rep-movsb memcpy block shared by every stub flavour that
// supports `Compress=true`:
//
//   - register setup: RAX = R15 (src), RBX = R15+ScratchDispFromText
//     (dst), RCX = CompressedSize.
//   - [EmitLZ4InflateInline] writes plaintext into the scratch buffer.
//   - memcpy back to R15: CLD; RSI = RBX, RDI = R15, RCX = OriginalSize;
//     REP MOVSB.
//
// After the block returns, [R15, R15+OriginalSize) holds plaintext
// .text. The block preserves R15 throughout (each ADD/SUB it does
// is on RBX/RCX/RDI/RSI — R15 stays as TextBase the entire time).
//
// errPrefix is folded into every wrapped error so the call-site
// flavour (EmitStub / EmitConvertedDLLStub / future EmitDLLStub
// LZ4 variant) shows up in the failure message instead of a
// generic "stage1: ...".
func emitLZ4DecompressBlock(b *amd64.Builder, opts EmitOptions, errPrefix string) error {
	if opts.CompressedSize == 0 || opts.OriginalSize == 0 || opts.ScratchDispFromText == 0 {
		return fmt.Errorf("%s: Compress=true but CompressedSize=%d OriginalSize=%d ScratchDispFromText=%d",
			errPrefix, opts.CompressedSize, opts.OriginalSize, opts.ScratchDispFromText)
	}

	// LZ4 register setup: src=.text, dst=scratch, srcSize=N.
	if err := b.MOV(amd64.RAX, baseReg); err != nil {
		return fmt.Errorf("%s: lz4 setup MOV RAX,R15: %w", errPrefix, err)
	}
	if err := b.LEA(amd64.RBX, amd64.MemOp{Base: baseReg, Disp: opts.ScratchDispFromText}); err != nil {
		return fmt.Errorf("%s: lz4 setup LEA RBX,scratch: %w", errPrefix, err)
	}
	if err := b.MOV(amd64.RCX, amd64.Imm(int64(opts.CompressedSize))); err != nil {
		return fmt.Errorf("%s: lz4 setup MOV RCX,CompressedSize: %w", errPrefix, err)
	}
	if err := EmitLZ4InflateInline(b); err != nil {
		return fmt.Errorf("%s: lz4 inflate inline: %w", errPrefix, err)
	}

	// memcpy plaintext back to R15.
	if err := b.RawBytes([]byte{0xfc}); err != nil { // cld (DF=0)
		return fmt.Errorf("%s: memcpy CLD: %w", errPrefix, err)
	}
	if err := b.MOV(amd64.RSI, amd64.RBX); err != nil {
		return fmt.Errorf("%s: memcpy MOV RSI,RBX: %w", errPrefix, err)
	}
	if err := b.MOV(amd64.RDI, baseReg); err != nil {
		return fmt.Errorf("%s: memcpy MOV RDI,R15: %w", errPrefix, err)
	}
	if err := b.MOV(amd64.RCX, amd64.Imm(int64(opts.OriginalSize))); err != nil {
		return fmt.Errorf("%s: memcpy MOV RCX,OriginalSize: %w", errPrefix, err)
	}
	if err := b.RawBytes([]byte{0xf3, 0xa4}); err != nil { // rep movsb
		return fmt.Errorf("%s: memcpy REP MOVSB: %w", errPrefix, err)
	}
	return nil
}
