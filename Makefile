OBJCOPY ?= llvm-objcopy

LIBDIR ?= $(CURDIR)/../tkey-libs

CC = clang

INCLUDE=$(LIBDIR)/include

# If you want libcommon's qemu_puts() et cetera to output something on our QEMU
# debug port, remove -DNODEBUG below
CFLAGS = -target riscv32-unknown-none-elf -march=rv32iczmmul -mabi=ilp32 -mcmodel=medany \
   -static -std=gnu99 -O2 -ffast-math -fno-common -fno-builtin-printf \
   -fno-builtin-putchar -nostdlib -mno-relax -flto -g \
   -Wall -Werror=implicit-function-declaration \
   -I $(INCLUDE) -I $(LIBDIR)  \
   -DNODEBUG

AS = clang
ASFLAGS = -target riscv32-unknown-none-elf -march=rv32iczmmul -mabi=ilp32 -mcmodel=medany -mno-relax

LDFLAGS=-T $(LIBDIR)/app.lds -L $(LIBDIR) -lcommon -lcrt0

# Check for OS, if not macos assume linux
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
	shasum = shasum -a 512
	BUILD_CGO_ENABLED ?= 1
else
	shasum = sha512sum
	BUILD_CGO_ENABLED ?= 0
endif

.PHONY: all
all: app/app.bin tkey-totp


podman:
	podman run --rm --mount type=bind,source=$(CURDIR),target=/src --mount type=bind,source=$(CURDIR)/../tkey-libs,target=/tkey-libs -w /src -it ghcr.io/tillitis/tkey-builder:2 make -j


# Turn elf into bin for device
%.bin: %.elf
	$(OBJCOPY) --input-target=elf32-littleriscv --output-target=binary $^ $@
	chmod a-x $@

check-hash: app/app.bin
	cd app && $(shasum) -c app.bin.sha512

# Random number generator app
OBJS=app/main.o app/app_proto.o app/cspring.o
app/app.elf: $(OBJS)
	$(CC) $(CFLAGS) $(OBJS) $(LDFLAGS) -L $(LIBDIR) -lmonocypher -o $@
$(OBJS): $(INCLUDE)/tkey/tk1_mem.h app/app_proto.h

# Uses ../.clang-format
FMTFILES=random-generator/*.[ch]

.PHONY: fmt
fmt:
	clang-format --dry-run --ferror-limit=0 $(FMTFILES)
	clang-format --verbose -i $(FMTFILES)

.PHONY: checkfmt
checkfmt:
	clang-format --dry-run --ferror-limit=0 --Werror $(FMTFILES)

TKEY_TOTP_VERSION ?= $(shell git describe --dirty --always | sed -n "s/^v\(.*\)/\1/p")

# .PHONY to let go-build handle deps and rebuilds
.PHONY: tkey-random-generator
tkey-totp: app/app.bin
	cp -af app/app.bin cmd/tkey-totp/app.bin
	CGO_ENABLED=$(BUILD_CGO_ENABLED) go build -ldflags "-X main.version=$(TKEY_TOTP_VERSION)" -trimpath -o tkey-totp ./cmd/tkey-totp



.PHONY: clean
clean:
	rm -f app/app.bin app/app.elf $(OBJS) \
	tkey-totp cmd/tkey-totp/app.bin


.PHONY: lint
lint:
	$(MAKE) -C gotools
	GOOS=linux   golangci-lint run
	GOOS=windows golangci-lint run

