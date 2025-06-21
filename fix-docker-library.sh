#!/bin/bash

echo "Docker kütüphanesi yeniden yükleniyor..."

# 1. Mevcut go.mod ve go.sum dosyalarını yedekle
cp go.mod go.mod.backup
cp go.sum go.sum.backup 2>/dev/null || true

# 2. Docker ile ilgili tüm bağımlılıkları kaldır
echo "Docker bağımlılıkları kaldırılıyor..."
go mod edit -droprequire github.com/docker/docker
go mod edit -droprequire github.com/docker/go-connections
go mod edit -droprequire github.com/docker/distribution

# 3. go.mod'u temizle
go mod tidy

# 4. Modül önbelleğini temizle
echo "Modül önbelleği temizleniyor..."
go clean -modcache

# 5. Docker kütüphanelerini uyumlu versiyonlarla ekle
echo "Docker kütüphaneleri yeniden ekleniyor..."

# Docker SDK'yı ekle (daha eski ama stabil versiyon)
go get github.com/docker/docker@v20.10.24+incompatible
go get github.com/docker/go-connections@v0.4.0

# Replace directive'leri ekle
go mod edit -replace=github.com/docker/distribution=github.com/distribution/distribution@v2.8.2+incompatible

# 6. Eksik bağımlılıkları indir
echo "Bağımlılıklar indiriliyor..."
go mod download

# 7. go.mod'u düzenle
go mod tidy

# 8. Build'i test et
echo "Build test ediliyor..."
make build

echo "İşlem tamamlandı!"