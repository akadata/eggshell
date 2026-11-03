#!/bin/sh
#
# Builds the vDSO shared objects embedded in the kernel (vdso_<arch>.so).
set -e

cd "$(dirname "$0")"

CFLAGS="-O2 -fPIC -ffreestanding -fno-stack-protector -fno-asynchronous-unwind-tables -nostdlib -Wall -Werror"
LDFLAGS="-shared -Wl,-T,vdso.lds -Wl,--hash-style=both -Wl,-soname=linux-vdso.so.1 -Wl,--no-undefined -Wl,-z,max-page-size=4096 -Wl,--build-id=none"

build() {
	cc=$1 arch=$2 ver=$3 extra=$4
	cat > vdso_$arch.ver <<EOF
$ver {
	global:
		__vdso_clock_gettime;
		__vdso_gettimeofday;
		__vdso_time;
	local: *;
};
EOF
	$cc $CFLAGS $extra $LDFLAGS -Wl,--version-script=vdso_$arch.ver -o vdso_$arch.so vdso.c
	rm vdso_$arch.ver

	# the image must be position independent without dynamic relocations
	if ${5}readelf -r vdso_$arch.so | grep -q "Relocation section"; then
		echo "vdso_$arch.so: unexpected dynamic relocations" >&2
		exit 1
	fi

	echo "vdso_$arch.so: $(wc -c < vdso_$arch.so) bytes"
}

# version nodes as exported by Linux and looked up by musl
build "${CC_AMD64:-gcc}" amd64 LINUX_2.6 "-mno-sse -mno-mmx -mno-red-zone" ""
