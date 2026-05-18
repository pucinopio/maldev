/* loader.ld — linker script for the PIC x86 BOF loader shellcode.
 *
 * Produces a single flat .text blob suitable for `objcopy -O binary
 * -j .text`. The loader_entry function MUST sit at offset 0 of the
 * extracted blob; that's what CreateRemoteThread jumps to.
 *
 * Sources for the entry-first ordering:
 *   - The function lives in section .text.entry (annotated in
 *     loader.c). KEEP() prevents --gc-sections from dropping it.
 *   - .text.entry is listed first inside the .text output section
 *     so the linker places it at the lowest address.
 */

ENTRY("_loader_entry@4")

SECTIONS
{
    . = 0;
    .text : {
        KEEP(*(.text.entry))
        *(.text)
        *(.text.*)
        *(.rdata*)
        *(.rodata*)
        *(.data*)
    }

    /DISCARD/ : {
        *(.bss*)
        *(.comment*)
        *(.eh_frame*)
        *(.note*)
        *(.idata*)
        *(.reloc*)
        *(.pdata*)
        *(.xdata*)
        *(.rsrc*)
        *(.tls*)
        *(.CRT*)
        *(.debug*)
    }
}
