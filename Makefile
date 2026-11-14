VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOFLAGS_REL := -trimpath -ldflags "$(LDFLAGS)"
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64

.PHONY: test build install build-all package clean

test:
	go test -race ./...

build:
	CGO_ENABLED=0 go build $(GOFLAGS_REL) -o optiimg ./cmd/optiimg

install:
	CGO_ENABLED=0 go install $(GOFLAGS_REL) ./cmd/optiimg

build-all:
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; [ $$os = windows ] && ext=".exe"; \
		echo "build $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(GOFLAGS_REL) -o dist/optiimg_$${os}_$${arch}$$ext ./cmd/optiimg || exit 1; \
	done

package: clean build-all
	@cd dist && for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; name=optiimg_$(VERSION)_$${os}_$${arch}; \
		mkdir -p stage; \
		if [ $$os = windows ]; then \
			cp optiimg_$${os}_$${arch}.exe stage/optiimg.exe && \
			(cd stage && zip -q ../$$name.zip optiimg.exe); \
		else \
			cp optiimg_$${os}_$${arch} stage/optiimg && \
			tar -C stage -czf $$name.tar.gz optiimg; \
		fi; \
		rm -r stage; \
	done; \
	shasum -a 256 *.tar.gz *.zip > checksums.txt; \
	cat checksums.txt

clean:
	rm -r -f optiimg dist
