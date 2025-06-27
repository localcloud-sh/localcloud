# scripts/build.sh
#!/bin/bash
VERSION=${VERSION:-$(git describe --tags --always --dirty)}
COMMIT=$(git rev-parse HEAD)
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags for smaller binary
LDFLAGS="-s -w \
  -X main.Version=$VERSION \
  -X main.Commit=$COMMIT \
  -X main.BuildDate=$BUILD_DATE"

# Build for all platforms
build_all() {
  # macOS
  GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/localcloud-darwin-amd64 ./cmd/localcloud
  GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/localcloud-darwin-arm64 ./cmd/localcloud

  # Linux
  GOOS=linux GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/localcloud-linux-amd64 ./cmd/localcloud
  GOOS=linux GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/localcloud-linux-arm64 ./cmd/localcloud

  # Windows
  GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/localcloud-windows-amd64.exe ./cmd/localcloud
}

# Create universal binary for macOS
create_universal_binary() {
  lipo -create -output dist/localcloud-darwin-universal \
    dist/localcloud-darwin-amd64 \
    dist/localcloud-darwin-arm64
}

# Create tarballs
create_archives() {
  cd dist
  tar -czf localcloud-$VERSION-darwin-amd64.tar.gz localcloud-darwin-amd64
  tar -czf localcloud-$VERSION-darwin-arm64.tar.gz localcloud-darwin-arm64
  tar -czf localcloud-$VERSION-darwin-universal.tar.gz localcloud-darwin-universal
  tar -czf localcloud-$VERSION-linux-amd64.tar.gz localcloud-linux-amd64
  tar -czf localcloud-$VERSION-linux-arm64.tar.gz localcloud-linux-arm64
  zip localcloud-$VERSION-windows-amd64.zip localcloud-windows-amd64.exe
  cd ..
}

# Generate checksums
generate_checksums() {
  cd dist
  shasum -a 256 *.tar.gz *.zip > checksums.txt
  cd ..
}

# Main
build_all
create_universal_binary
create_archives
generate_checksums