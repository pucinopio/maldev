/*
 * noop.x86.c — minimal i386 BOF fixture for slice 1.d step 1.b.
 *
 * The BOF has no Beacon API imports and no cross-section
 * references. It simply marks the args/alen as used and returns.
 * Verifies the loader's COFF parse + section copy + entry-call
 * path without exercising relocations or the Beacon API resolver
 * (those land in step 1.c + 1.d).
 *
 * Build (committed as noop.x86.o):
 *   i686-w64-mingw32-gcc -m32 -c noop.x86.c -o noop.x86.o
 */

void go(char *args, int alen)
{
    (void)args;
    (void)alen;
}
