# Go API Starter

Yeni başlayanlar için hazırlanmış, tamamen Docker ile çalışan sade bir Go REST API örneği.

## Bu projede neler var?

- Go `net/http` ile yazılmış basit API
- PostgreSQL bağlantısı
- Docker Compose ile tek komutla çalışma
- Swagger UI dokümantasyonu
- `/health` endpoint'i ve container healthcheck
- Users için örnek CRUD endpoint'leri
- JSON formatında request logları
- HTTP seviyesinde integration testler

## 1 dakikada başlat

Projeyi çalıştırmak için:

Windows:

```bash
scripts\\dev-up.bat
```

Unix benzeri ortamlar:

```bash
sh scripts/dev-up.sh
```

Tarayıcıdan aç:

- API: `http://localhost:8080`
- Swagger UI: `http://localhost:8080/swagger/index.html`

Durdurmak için:

Windows:

```bash
scripts\\dev-down.bat
```

Unix benzeri ortamlar:

```bash
sh scripts/dev-down.sh
```

## Sık kullanılan komutlar

### Uygulamayı başlat

Windows:

```bash
scripts\\dev-up.bat
```

Unix benzeri ortamlar:

```bash
sh scripts/dev-up.sh
```

### Logları izle

```bash
docker compose logs -f
```

### Opsiyonel: air ile hot reload geliştirme modu

Kod değiştikçe container içinde uygulamanın otomatik yeniden derlenmesini istersen aşağıdaki scriptleri kullanabilirsin.

Windows:

```bash
scripts\\dev-up-air.bat
```

Unix benzeri ortamlar:

```bash
sh scripts/dev-up-air.sh
```

İstersen aynı komutu doğrudan da çalıştırabilirsin:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up
```

Bu modda `air` container içinde kurulur ve dosya değişikliklerini izleyerek API'yi yeniden başlatır.

Durdurmak için:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml down
```

Ne zaman kullanmalı?

- sık sık handler veya store kodu değiştiriyorsan
- her değişiklikte image rebuild beklemek istemiyorsan

Ne zaman kullanmayabilirsin?

- Docker mantığını ilk kez öğreniyorsan
- daha sade bir başlangıç akışı istiyorsan

Ana öneri yine normal `scripts\\dev-up.bat` veya `sh scripts/dev-up.sh` akışıdır. `air` modu tamamen opsiyoneldir.

### Container durumunu kontrol et

```bash
docker compose ps
```

`api` servisi `healthy` görünüyorsa uygulama çalışıyor ve `/health` endpoint'i başarılı cevap veriyor demektir.

### Test veritabanını başlat

Windows:

```bash
scripts\\test-db-up.bat
```

Unix benzeri ortamlar:

```bash
sh scripts/test-db-up.sh
```

### Test veritabanını durdur

Windows:

```bash
scripts\\test-db-down.bat
```

Unix benzeri ortamlar:

```bash
sh scripts/test-db-down.sh
```

## Endpoint'ler

### GET /health

Uygulamanın çalıştığını kontrol eder.

Örnek cevap:

```json
{
  "message": "API is running",
  "status": "ok"
}
```

### GET /users

Tüm kullanıcıları döner.

### GET /users/{id}

Tek bir kullanıcıyı döner.

### POST /users

Yeni kullanıcı oluşturur.

Örnek istek gövdesi:

```json
{
  "name": "Ada Lovelace",
  "email": "ada@example.com"
}
```

### PUT /users/{id}

Var olan kullanıcıyı günceller.

Örnek istek gövdesi:

```json
{
  "name": "Ada Byron",
  "email": "ada.byron@example.com"
}
```

### DELETE /users/{id}

Kullanıcıyı siler.

### Hata davranışı

Aynı `email` ile ikinci kez kullanıcı oluşturulursa veya başka bir kullanıcının email'i ile güncelleme yapılırsa API `409 Conflict` döner.

## curl ile hızlı deneme

Yeni başlayanlar için terminalden doğrudan deneyebileceğin birkaç örnek:

### 1. Health kontrolü

```bash
curl http://localhost:8080/health
```

### 2. Kullanıcı oluştur

```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Ada Lovelace","email":"ada@example.com"}'
```

### 3. Tüm kullanıcıları listele

```bash
curl http://localhost:8080/users
```

### 4. Kullanıcı güncelle

```bash
curl -X PUT http://localhost:8080/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Ada Byron","email":"ada.byron@example.com"}'
```

Not: PowerShell kullanıyorsan `curl` bazen `Invoke-WebRequest` davranışı gösterebilir. Gerekirse `curl.exe` yazabilirsin.

## Environment değişkenleri

| Değişken | Nerede kullanılır? | Örnek değer | Açıklama |
|---|---|---|---|
| `PORT` | API container içinde | `8080` | API'nin dinlediği port |
| `DATABASE_URL` | API uygulaması ve PostgreSQL integration testleri | `postgres://postgres:postgres@postgres:5432/go_lang?sslmode=disable` | Uygulamanın PostgreSQL bağlantı adresi |

## Docker ağ mantığı

| Durum | Hostname | Port | Açıklama |
|---|---|---|---|
| API container → ana veritabanı | `postgres` | `5432` | Docker Compose ağı içinde servis adına göre bağlanır |
| Bilgisayarın kendisi → ana veritabanı | `localhost` | `5432` | Host makineden doğrudan veritabanına bağlanmak için kullanılır |
| Bilgisayarın kendisi → test veritabanı | `localhost` | `5433` | Ayrı `docker-compose.test.yml` akışı için kullanılır |

Kısacası: container içinden başka bir servise bağlanırken `localhost` değil, Compose servis adı kullanılır.

## Swagger kullanımı

Swagger UI adresi:

```text
http://localhost:8080/swagger/index.html
```

Swagger dokümantasyonu handler dosyalarındaki annotation'lardan üretilir.

Docs üretmek için:

```bash
go generate ./cmd/api
```

Windows için istersen:

```bash
swagger-generate.bat
```

Unix benzeri ortamlar için:

```bash
sh swagger-generate.sh
```

### Swagger annotation şablonu

Yeni bir endpoint eklerken aşağıdaki şablonu kopyalayıp uyarlayabilirsin:

```go
// CreateThing godoc
// @Summary Create thing
// @Description Yeni bir kayıt oluşturur
// @Tags things
// @Accept json
// @Produce json
// @Param thing body model.CreateThingRequest true "New thing"
// @Success 201 {object} model.Thing
// @Failure 400 {object} model.ErrorResponse
// @Failure 409 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /things [post]
func (h ThingHandler) CreateThing(w http.ResponseWriter, r *http.Request) {
	// handler body
}
```

Path parametresi olan örnek:

```go
// GetThingByID godoc
// @Summary Get thing by ID
// @Description ID ile kayıt getirir
// @Tags things
// @Produce json
// @Param id path int true "Thing ID"
// @Success 200 {object} model.Thing
// @Failure 400 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /things/{id} [get]
func (h ThingHandler) GetThingByID(w http.ResponseWriter, r *http.Request) {
	// handler body
}
```

## Testler

### Mevcut integration testleri çalıştır

Bellek içi testler:

```bash
go test ./...
```

### PostgreSQL ile integration test çalıştır

Önce test veritabanını ayağa kaldır:

Windows:

```bash
scripts\\test-db-up.bat
```

Unix benzeri ortamlar:

```bash
sh scripts/test-db-up.sh
```

Sonra testleri çalıştır:

```bash
set "DATABASE_URL=postgres://postgres:postgres@localhost:5433/go_lang_test?sslmode=disable"
go test ./...
```

## İlk katkı nasıl yapılır?

Projeye ilk kez küçük bir katkı vermek istiyorsan şu sırayı izle:

1. Uygulamayı ayağa kaldır
2. Swagger UI'dan endpoint'leri kontrol et
3. Küçük bir handler veya response değişikliği yap
4. Gerekirse Swagger docs üret: `go generate ./cmd/api`
5. Testleri çalıştır: `go test ./...`

İlk katkı için iyi adaylar:

- yeni bir endpoint eklemek
- hata mesajını iyileştirmek
- yeni bir request/response modeli eklemek
- README'ye örnek eklemek

## Proje yapısı

İstek akışı basitçe şöyledir:

```text
HTTP Request
   -> internal/server
   -> internal/handler
   -> internal/store üzerinden UserStore arayüzü
      -> memory store
      -> postgres store
   -> internal/response
   -> HTTP Response
```

Bu sayede handler katmanı verinin nerede tutulduğunu bilmeden aynı arayüz ile çalışır.

```text
.
├── cmd/api/main.go                 # Uygulama başlangıcı
├── internal/database/              # PostgreSQL bağlantısı ve migration
├── internal/handler/               # HTTP handler'lar
├── internal/model/                 # Request/response modelleri
├── internal/server/                # Router, middleware, testler
├── internal/store/                 # Memory ve PostgreSQL store'ları
├── docs/                           # Üretilen Swagger dosyaları
├── docker-compose.yml              # Uygulama + PostgreSQL
├── docker-compose.test.yml         # Test PostgreSQL servisi
├── Dockerfile                      # API image tanımı
└── README.md
```

## Notlar

- Uygulama tamamen Docker üzerinden çalışacak şekilde hazırlanmıştır.
- Ana veritabanı servisi `postgres` adıyla compose ağı içinde çalışır.
- Test veritabanı ayrı compose dosyasında `5433` portundan açılır.
- Request logları JSON olarak yazılır.
- Swagger dokümantasyonu `go generate ./cmd/api` ile yeniden üretilebilir.

## Bu proje neden sade tutuldu?

Amaç, yeni başlayan birinin:

- klasör yapısını hızlı anlaması
- CRUD akışını görmesi
- Docker ile geliştirme ortamını ayağa kaldırması
- Swagger ve test mantığını öğrenmesi

Bu yüzden gereksiz soyutlama ve karmaşıklık özellikle eklenmedi.
