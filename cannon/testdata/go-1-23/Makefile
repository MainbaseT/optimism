all: elf

.PHONY: elf64
elf64: $(patsubst %/go.mod,bin/%.64.elf,$(wildcard */go.mod))

.PHONY: elf
elf: elf64

.PHONY: dump
dump: $(patsubst %/go.mod,bin/%.dump,$(wildcard */go.mod))

.PHONY: clean
clean:
	@[ -d bin ] && find bin -maxdepth 1 -type f -delete

bin:
	mkdir bin

# take any directory with a go mod, and build an ELF
# verify output with: readelf -h bin/<name>.elf
# result is mips64, big endian, R3000
bin/%.64.elf: bin
	cd $(@:bin/%.64.elf=%) && GOOS=linux GOARCH=mips64 GOMIPS64=softfloat go build -o ../$@ .

# take any ELF and dump it
# TODO: currently have the little-endian toolchain, but should use the big-endian one. The -EB compat flag works though.
bin/%.dump: bin
	mipsel-linux-gnu-objdump -D --disassembler-options=no-aliases --wide --source -m mips:3000 -EB $(@:%.dump=%.elf) > $@
