/* vDSO layout: a single read+execute segment at address 0, the vvar data page
 * mapped by the kernel immediately before it. */

SECTIONS
{
	PROVIDE_HIDDEN(__vvar = . - 4096);

	. = SIZEOF_HEADERS;

	.hash		: { *(.hash) }			:text
	.gnu.hash	: { *(.gnu.hash) }
	.dynsym		: { *(.dynsym) }
	.dynstr		: { *(.dynstr) }
	.gnu.version	: { *(.gnu.version) }
	.gnu.version_d	: { *(.gnu.version_d) }
	.gnu.version_r	: { *(.gnu.version_r) }

	.dynamic	: { *(.dynamic) }		:text	:dynamic

	.rodata		: { *(.rodata*) *(.srodata*) }	:text
	.text		: { *(.text*) }
	.got		: { *(.got.plt) *(.got) }

	/DISCARD/	: {
		*(.data*) *(.sdata*) *(.bss*) *(.sbss*) *(.plt*)
		*(.eh_frame*) *(.note*) *(.comment) *(.riscv.attributes)
	}
}

PHDRS
{
	text		PT_LOAD		FLAGS(5) FILEHDR PHDRS;	/* PF_R|PF_X */
	dynamic		PT_DYNAMIC	FLAGS(4);		/* PF_R */
}
