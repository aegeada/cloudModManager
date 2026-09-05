# Cloud Mod Manager (`cmm`) — Kurulum ve Kullanım Rehberi

**Cloud Mod Manager (`cmm`)**, modern Minecraft sunucuları ve modpack geliştiricileri için tasarlanmış; Modrinth entegrasyonuna, özyinelemeli bağımlılık çözümlemeye, çok kaynaklı senkronizasyon motorlarına ve zengin bir Terminal Kullanıcı Arayüzüne (TUI) sahip yeni nesil bir mod yöneticisidir.

---

## 📑 İçindekiler
1. [Kurulum Rehberi (Installation)](#1-kurulum-rehberi-installation)
   - [Önceden Derlenmiş İkilileri Kullanma (Tavsiye Edilen)](#önceden-derlenmiş-ikilileri-kullanma-tavsiye-edilen)
   - [Kaynaktan Derleme (Build from Source)](#kaynaktan-derleme-build-from-source)
2. [5 Dakikada Hızlı Başlangıç (Quickstart)](#2-5-dakikada-hızlı-başlangıç-quickstart)
3. [Kapsamlı Komut Rehberi (CLI Reference)](#3-kapsamlı-komut-rehberi-cli-reference)
   - [1. Proje Başlatma: `cmm init`](#1-proje-başlatma-cmm-init)
   - [2. Mod Arama: `cmm search`](#2-mod-arama-cmm-search)
   - [3. Mod Yükleme: `cmm add`](#3-mod-yükleme-cmm-add)
   - [4. Mod Listeleme: `cmm list`](#4-mod-listeleme-cmm-list)
   - [5. Mod Güncelleme: `cmm update`](#5-mod-güncelleme-cmm-update)
   - [6. Mod Sabitleme (Pinning): `cmm pin` & `cmm unpin`](#6-mod-sabitleme-pinning-cmm-pin--cmm-unpin)
   - [7. Mod Silme: `cmm remove`](#7-mod-silme-cmm-remove)
   - [8. Yükleyici Yönetimi: `cmm loader`](#8-yükleyici-yönetimi-cmm-loader)
   - [9. Senkronizasyon Motorları: `cmm sync`](#9-senkronizasyon-motorları-cmm-sync)
   - [10. HTTP Senkronizasyon Sunucusu: `cmm serve`](#10-http-senkronizasyon-sunucusu-cmm-serve)
   - [11. Modpack Dışa Aktarma: `cmm export`](#11-modpack-dışa-aktarma-cmm-export)
4. [Görsel Terminal Arayüzü: `cmm tui`](#4-görsel-terminal-arayüzü-cmm-tui)
5. [Yapılandırma Dosyaları (`cmm.toml` & `cmm.lock`)](#5-yapılandırma-dosyaları-cmmtoml--cmmlock)
6. [Gerçek Hayat Senaryoları & İpuçları](#6-gerçek-hayat-senaryoları--ipuçları)

---

## 1. Kurulum Rehberi (Installation)

### Otomatik Kurulum (Self-Installation — Sıfır Zahmet!)
`cmm` ikilisini ilk kez herhangi bir klasörden çalıştırdığınızda (örn. `./cmm-linux-arm64`), kendini otomatik olarak kullanıcı dizininizdeki `~/.local/bin/cmm` konumuna kopyalar ve gerekiyorsa `~/.bashrc` dosyanıza PATH değişkenini ekler:

```bash
# Sadece indirdiğiniz ikiliyi çalıştırın:
./cmm-linux-arm64

# Çıktı:
# ✨ [Auto-Setup] Successfully installed 'cmm' to ~/.local/bin/cmm
# 💡 You can now run 'cmm' directly from any folder in your terminal!

# Artık her yerden sadece 'cmm' yazarak kullanabilirsiniz:
cmm --help
```

---

### Önceden Derlenmiş İkilileri Kullanma (Tavsiye Edilen)
Projenin `bin/` dizini altında tüm platformlar için bağımsız (CGO gerektirmeyen, statik) ikililer hazır bulunmaktadır:

| Platform | Mimari | Dosya Yolu |
|---|---|---|
| **Linux (Oracle Cloud Always Free)** | **ARM64 (Ampere A1)** | `bin/cmm-linux-arm64` |
| **Linux (Standart Sunucu/PC)** | **AMD64 (x86_64)** | `bin/cmm-linux-amd64` |
| **macOS (Apple Silicon)** | **ARM64 (M1/M2/M3/M4)** | `bin/cmm-darwin-arm64` |
| **macOS (Intel)** | **AMD64 (x86_64)** | `bin/cmm-darwin-amd64` |
| **Windows** | **AMD64 (x86_64)** | `bin/cmm-windows-amd64.exe` |

#### Linux / Oracle Cloud ARM64 Kurulumu:
```bash
# 1. İkiliyi sistem genelinde kullanılabilir yapın:
sudo cp bin/cmm-linux-arm64 /usr/local/bin/cmm
sudo chmod +x /usr/local/bin/cmm

# 2. Kurulumu doğrulayın:
cmm --help
```

#### macOS Kurulumu:
```bash
sudo cp bin/cmm-darwin-arm64 /usr/local/bin/cmm
sudo chmod +x /usr/local/bin/cmm
cmm --help
```

#### Windows Kurulumu (PowerShell):
`bin/cmm-windows-amd64.exe` dosyasını `cmm.exe` olarak adlandırıp ortam değişkenlerinizdeki (PATH) bir klasöre taşıyabilirsiniz.

---

### Kaynaktan Derleme (Build from Source)
Go 1.22+ yüklü bir ortamda:

```bash
# Projeyi klonlayın ve dizine girin:
git clone https://github.com/your-org/cloudModManager.git
cd cloudModManager

# Yerel ikiliyi derleyin:
make build

# Tüm platformlar için aynı anda derleyin:
make cross-compile

# Tüm test paketlerini çalıştırın (149 Test):
make test-all
```

---

## 2. 5 Dakikada Hızlı Başlangıç (Quickstart)

```bash
# 1. Sunucu dizininde cmm'i başlatın:
cmm init --name "MyServer" --mc-version "1.21.1" --loader "fabric" --side "server"

# 2. Fabric Loader'ı kurun:
cmm loader install fabric --version 0.19.3

# 3. İhtiyacınız olan modları arayın ve kurun (bağımlılıkları otomatik çözer):
cmm search "lithium"
cmm add lithium
cmm add ferrite-core

# 4. Kritik bir modun sürümünü sabitleyin (otomatik güncellenmesin):
cmm pin lithium

# 5. Tüm modların güncellemelerini kontrol edin:
cmm update

# 6. Görsel Terminal Kullanıcı Arayüzünü (TUI) başlatın:
cmm tui
```

---

## 3. Kapsamlı Komut Rehberi (CLI Reference)

### 1. Proje Başlatma: `cmm init`
Mevcut dizinde `cmm.toml` yapılandırma dosyasını oluşturur.

```bash
# Etkileşimli (Soru-Cevap) Mod:
cmm init

# Bayraklar ile Otomatik (Script/CI) Mod:
cmm init \
  --name "SurvivalServer" \
  --mc-version "1.21.1" \
  --loader "fabric" \
  --side "server" \
  --mods-dir "mods"
```
- `--side`: `server` (istemci modlarını atlar), `client` veya `both`.

---

### 2. Mod Arama: `cmm search`
Modrinth API üzerinden yapılandırmanızdaki Minecraft sürümü ve yükleyiciye göre filtrelenmiş arama yapar.

```bash
# Basit arama:
cmm search "optimization"

# Sonuç sayısını sınırlandırma:
cmm search "fabric api" --limit 5
```

---

### 3. Mod Yükleme: `cmm add` / `cmm install`
Modrinth üzerinden mod indirir, bağımlılıkları özyinelemeli (recursive) çözer ve `cmm.lock` dosyasını günceller. `cmm install`, `cmm add` komutunun tam eşdeğeri (alias) olarak kullanılabilir.

- **Tek Mod ve Sürüm Belirtilmemişse (Etkileşimli Sayfalama)**:
  Seçilen kararlılık kanalındaki son 10 sürümü numaralandırarak listeler. `[n]` sonraki sayfa, `[p]` önceki sayfa, `[q]` iptal. Numara seçildiğinde indirme öncesi dosya adıyla onay ister.
  ```bash
  cmm install sodium
  ```
- **Spesifik Sürüm Kurulumu (`-v` / `--version`)**:
  Doğrudan belirtilen sürümü hedefler ve dosya adıyla onay ister:
  ```bash
  cmm install sodium -v "0.5.8"
  ```
- **Çoklu Mod Kurulumu (Toplu Kurulum)**:
  Boşlukla ayrılmış birden fazla slug girildiğinde sayfalama devre dışı bırakılır, her mod için son uyumlu sürüm çözümlenir, tek bir toplu özet tablo basılır ve tek onay ile hepsi kurulur:
  ```bash
  cmm install sodium iris lithium
  ```
- **Sürüm Kararlılık Kanalı Filtresi (`--channel`)**:
  Varsayılan kanal `release`'dir. `beta` (release + beta) veya `alpha` (tüm sürümler) seçilebilir:
  ```bash
  cmm install sodium --channel beta
  ```
- **Zaten Kurulu Mod Uyarısı ve Değiştirme (Replacement)**:
  Eğer mod zaten kuruluysa, mevcut ve hedef sürüm gösterilerek onay istenir (`Replace installed version with <targetVer>? [y/N]: `). Onay verilirse eski `.jar` diskten silinir ve yeni sürüm kurulur.

---

### 4. Mod Listeleme: `cmm list`
Kurulu olan tüm modları, versiyonlarını, mod sabitleme (`[PINNED]`), pasiflik (`[DISABLED]`) ve güncelleme (`[UPDATE AVAILABLE]`) durumlarını gösterir.

```bash
# Tablo Görünümü:
cmm list

# JSON Çıktısı (Script ve Otomasyonlar için):
cmm list --json
```

---

### 5. Mod Güncelleme: `cmm update`
Kurulu modların Modrinth üzerindeki en güncel sürümlerini kontrol eder ve günceller.

- **Modrinth Slug Gösterimi**: Güncellenecek modların yanında slug bilgisi parantez içinde belirtilir: `- Sodium (sodium): 0.5.8 -> 0.5.11`.
- **Kararlılık Kanalı Filtresi (`--channel`)**: `--channel release` (varsayılan), `--channel beta` veya `--channel alpha`. Aktif kanal bilgisi komut çıktısının en başında `🔍 Checking for updates... [Channel: release]` olarak gösterilir.
- **Çoklu Slug ve Tek Onay**: Boşlukla ayrılmış modlar girilebilir (`cmm update sodium iris lithium`). Toplu özet listelenir ve tek bir `Apply updates? [y/N]: ` onayı ile hepsi güncellenir.
- **Loader Bildirimi**: Eğer daha yeni bir Loader sürümü varsa akış bozulmadan çıktı sonunda tek satırlık bildirim basılır (`Notice: A new Fabric Loader version is available (v0.19.5). Run: cmm loader update`).

```bash
# Tüm modları güncelleme (release kanalı):
cmm update

# Beta kanalında birden fazla spesifik modu güncelleme:
cmm update sodium iris --channel beta

# Sabitlenmiş (pinned) modları da zorla güncelleme:
cmm update --force
```

---

### 6. Mod Sabitleme (Pinning): `cmm pin` & `cmm unpin`
Modların `cmm update` veya `cmm sync` komutları çalıştırıldığında otomatik olarak yükseltilmesini engeller.

```bash
# Modun mevcut sürümünü sabitleme:
cmm pin lithium

# Modu belirli bir sürüme sabitleme:
cmm pin sodium --version 0.5.8

# Sabitlemeyi kaldırma (güncellemelere açma):
cmm unpin lithium
```

---

### 7. Mod Silme: `cmm remove`
Bir modu diskten ve kilit dosyasından siler. Eğer silinen modun bağımlılıkları başka bir mod tarafından kullanılmıyorsa (orphan), onları da temizleme seçeneği sunar.

```bash
# Modu silme:
cmm remove iris
```

---

### 8. Modları Aktif / Pasif Yapma: `cmm enable` & `cmm disable`
Modu tamamen silmeden geçici olarak kapatıp açmanızı sağlar:
- `cmm disable <mod>`: Mod dosyasını `mods/<mod>.jar.disabled` olarak yeniden adlandırır ve `cmm.lock` kilit dosyasında pasif (`disabled = true`) olarak işaretler. Minecraft sunucusu veya istemcisi bu modu yüklemez.
- `cmm enable <mod>`: Mod dosyasını tekrar `.jar` haline getirir ve kilit dosyasını günceller.
- **TUI Kısayolu**: `cmm tui` içerisindeyken listedeki herhangi bir modun üzerindeyken **`e`** veya **`Space` (Boşluk)** tuşuna basarak anında aktif/pasif yapabilirsiniz!

```bash
# Modu devre dışı bırakma (.jar.disabled):
cmm disable sodium

# Başka modlar buna bağımlı olsa bile zorla devre dışı bırakma:
cmm disable fabric-api --force

# Modu tekrar aktif hale getirme (.jar):
cmm enable sodium
```

---

### 9. Yükleyici Yönetimi & Güncelleme: `cmm loader`
Minecraft mod yükleyicilerini (Fabric, Forge, NeoForge, Quilt) listeler, kurar ve güvenli bir şekilde günceller.

- **Yükleyici Sürümlerini Listeleme**:
  ```bash
  cmm loader list fabric
  ```
- **Yükleyici Kurma**:
  ```bash
  cmm loader install fabric --version 0.19.3
  ```
- **Yükleyici Güncelleme & Süreç Güvenlik Kilidi (`cmm loader update`)**:
  En yeni kararlı loader sürümünü sorgular ve onayınızla `cmm.toml` dosyasını günceller.
  - **Süreç Güvenlik Kilidi (Process Safety Check)**: İşlem öncesinde arka planda çalışan Java / Minecraft sunucu süreci olup olmadığını denetler. Eğer sunucu aktifse güncelleme durdurulur (`Error: Server is currently running. Please stop the server before updating the loader (or use --force).`).
  ```bash
  cmm loader update
  # Zorla güncellemek için:
  cmm loader update --force
  ```

---

### 10. Minecraft Sürümü Yönetimi: `cmm mc-version`
`cmm.toml` dosyasındaki Minecraft sürümünü görüntüler veya güvenli bir şekilde günceller.

```bash
# Yapılandırılmış sürümü görüntüleme:
cmm mc-version
# Çıktı: Configured Minecraft version: 26.2

# Sürümü güncelleme (regex format doğrulaması ve kullanıcı onayı ile):
cmm mc-version "26.1.2"
# Soru: Change configured Minecraft version from 26.2 to 26.1.2? [y/N]: y
```

---

### 11. Sunucu Tarama ve Kesin Sürüm Tespiti: `cmm scan`
Mevcut bir Minecraft sunucu klasörünü baştan sona tarar ve ortamı otomatik olarak yapılandırır:
- **Kesin Sürüm Tespiti (Deterministic Version Detection)**:
  - Sunucu `server.jar` içindeki `version.json` dosyasını doğrudan ZIP reader ile açarak kesin Minecraft sürümünü (`id`) çıkarır.
  - İstemci ve modpack manifestolarını (`modrinth.index.json`, `instance.json`, vb.) okur.
  - Eğer kesin sürüm tespit edilemezse mod adlarından frekans oylamasıyla bulunan yaygın sürümü önerir (`Could not determine exact Minecraft version. Use detected version (26.2)? [Y/n]: `). Reddedilirse manuel giriş ister.
  - Aynı tespit ve onay mantığı Yükleyici (Loader) için de geçerlidir.
- **Loader Bildirimi**: Tarama bittiğinde yeni bir loader sürümü mevcutsa bildirim basar.

```bash
# Mevcut dizini tarama ve cmm dosyalarını üretme:
cmm scan

# Farklı bir sunucu dizinini tarama:
cmm scan /home/ubuntu/minecraft-server
```

---

### 12. Cloud Mod Manager Sürüm Denetimi ve Kendi Kendini Güncelleme: `cmm version`
Cloud Mod Manager'ın mevcut sürümünü gösterir, GitHub üzerindeki en son yayınlanan sürümü (`aegeada/cloudModManager`) sorgular ve yeni bir sürüm çıktığında tek onay ile binary'i otomatik olarak yerinde günceller:

```bash
# Sürümü ve güncelleme durumunu denetleme:
cmm version
# Çıktı:
# Cloud Mod Manager v0.1.0 (linux/amd64)
# 🔍 Checking for updates...
# ✅ Cloud Mod Manager is up to date (v0.1.0).

# Yeni bir sürüm varsa:
# 🚀 A new version of Cloud Mod Manager is available: v0.2.0 (Current: v0.1.0)
# Download and install v0.2.0 now? [y/N]: y
# ⬇️  Downloading and installing v0.2.0...
# ✨ Successfully updated Cloud Mod Manager to v0.2.0!

# Yalnızca güncelleme olup olmadığını kontrol etme (onay sormadan):
cmm version --check

# Sormadan otomatik güncelleme:
cmm version --yes
```

> **İpucu**: `cmm init` komutunu içinde önceden `.jar` modları olan bir klasörde çalıştırdığınızda, `cmm` mevcut modları otomatik olarak algılar ve `cmm.lock` dosyanızı hemen oluşturur.

---

### 10. Senkronizasyon Motorları: `cmm sync`

#### A. Yerel Dizin Senkronizasyonu (`cmm sync local`)
Yerel `mods/` klasörünüzdeki `.jar` dosyalarını tarar, SHA-512 hash'lerini Modrinth'te sorgular ve sıfırdan geçerli bir `cmm.lock` üretir (Tanınmayan özel modlar için uyarı verir, pinli durumları korur).
```bash
cmm sync local
cmm sync local --path /opt/minecraft/mods
```

#### B. Modrinth Modpack Eşitleme (`cmm sync --source modrinth`)
Resmi bir Modrinth `.mrpack` paketini (slug veya dosya yolu) sunucuya senkronize eder. `side = "server"` kurallarına göre istemci modlarını eler ve listede olmayan gereksiz JAR'ları temizler (Delta Sync).
```bash
# Modrinth slug üzerinden:
cmm sync --source modrinth --slug fabulously-optimized

# İndirilmiş .mrpack dosyası üzerinden:
cmm sync --source modrinth --file /tmp/my-modpack.mrpack
```

#### C. GitHub Depo Eşitleme (`cmm sync --source github`)
GitHub üzerinde saklanan bir sunucu reposundan `cmm.lock` ve `cmm.toml` dosyalarını çeker ve modları eşitler.
```bash
# Açık kaynak depo:
cmm sync --source github --repo my-org/minecraft-server-mods --branch main

# Özel (Private) depo:
cmm sync --source github --repo my-org/private-modpack --token ghp_yourSecretToken
```

#### D. Uzak HTTP Sunucu Eşitleme (`cmm sync --url`)
Çalışan bir `cmm serve` sunucusuna bağlanarak istemci/sunucu kilit dosyasını senkronize eder.
```bash
cmm sync --url http://192.168.1.100:8080 --token mySecretToken
```

---

---

### 11. İstemciden Sunucuya Uzaktan Dağıtım: `cmm push`
İstemci bilgisayarınızda (Windows, macOS, Linux) hazırladığınız modpack kilit dosyasını (`cmm.lock`) ve isteğe bağlı olarak sunucu konfigürasyonlarını (`config/`) tek komutla uzak sunucunuza aktarır. Uzak sunucu token'ı doğrular, konfigürasyonları Zip-Slip korumasıyla açar ve delta senkronizasyon motorunu tetikleyerek yeni modları sunucuya indirir, eski modları temizler.

```bash
# Sadece mod kilit dosyasını push'lama:
cmm push --url http://123.45.67.89:8080 --token mySecretToken

# Hem mod kilit dosyasını hem de config/ klasörünü sunucuya push'lama:
cmm push --url http://123.45.67.89:8080 --token mySecretToken --include-config

# Değişiklik yapmadan simüle etme (Dry-run):
cmm push --url http://123.45.67.89:8080 --token mySecretToken --dry-run
```

---

### 12. İstemci-Sunucu Uyum ve Sürüm Karşılaştırması: `cmm diff`
İstemci modlarınız ile uzak sunucudaki modları (veya iki farklı kilit dosyasını) 5 yönlü olarak karşılaştırır:
- `[OK]`: İstemci ve sunucuda sürümü ve hash'i birebir eşleşen modlar.
- `[MISMATCH]`: Her iki tarafta da kurulu olan ancak sürümleri farklı olan modlar.
- `[CLIENT]`: İstemciye özel optimizasyon/görsel modlar (Sodium, Iris vb. — sunucuda olmaması normal).
- `[SERVER]`: Sunucuya özel performans/yönetim modları.
- `[MISSING]`: İki taraftan birinde eksik olan zorunlu ortak modlar.

```bash
# İstemci ile uzak sunucuyu karşılaştırma:
cmm diff --url http://123.45.67.89:8080 --token mySecretToken

# İki kilit dosyasını yerel olarak karşılaştırma:
cmm diff cmm.lock /tmp/server-cmm.lock

# JSON formatında çıktı alma (CI/CD otomasyonları için):
cmm diff --url http://123.45.67.89:8080 --token mySecretToken --json
```

---

### 13. İstemci Başlatıcıları (Launcher) Yönetimi: `cmm launcher`
Bilgisayarınızda kurulu olan Minecraft başlatıcılarını (Standart `.minecraft`, **Prism Launcher**, **Modrinth App**, **CurseForge**) otomatik olarak algılar ve modpack'inizi doğrudan o başlatıcının instance klasörüne aktarır.

```bash
# Algılanan başlatıcıları ve instance'ları listeleme:
cmm launcher list

# Modpack'i doğrudan seçilen başlatıcı instance'ına eşitleme:
cmm launcher sync "Prism - Fabric 1.21.1"
```

---

### 14. HTTP Senkronizasyon Sunucusu: `cmm serve`
Sunucunuzdaki aktif `cmm.lock` dosyasını HTTP üzerinden yayınlar ve `POST /push` ile uzaktan gelen modpack/config güncellemelerini kabul eder.

```bash
# Varsayılan portta (8080) başlatma:
cmm serve

# Özel port ve Bearer Token kimlik doğrulaması ile:
cmm serve --port 8085 --token "super-secret-sync-key"
```
- `GET /lock`: Kilit dosyasını döner (`Authorization: Bearer <token>` gerektirir).
- `POST /push`: Uzaktan `cmm push` ile gelen modpack ve `config.zip` arşivini alıp sunucuyu günceller.
- `GET /health`: Canlılık kontrolü.
- `SIGINT`/`SIGTERM` (Ctrl+C): 2 saniye içinde açık soket bırakmadan zarif kapanır (Graceful Shutdown).

---

### 15. Modpack Dışa Aktarma: `cmm export`
Kurulu modlarınızı dağıtılabilir paket formatlarına dönüştürür.

```bash
# 1. Standart Modrinth .mrpack ZIP Arşivi Üretme:
# (modrinth.index.json, hash'ler, indirme linkleri ve config/ overrides içerir)
cmm export --format mrpack --output dist/MyPack.mrpack --name "My Modpack" --version-id "1.0.0"

# 2. Temiz GitHub Repo Konfigürasyonu Üretme:
# (Hassas tokenları temizlenmiş cmm.toml ve cmm.lock üretir)
cmm export --format github --output dist/github-repo/
```

---

## 4. Görsel Terminal Arayüzü: `cmm tui`

`cmm tui` komutu ile terminalinizde modern, fare ve klavye destekli bir kontrol paneli açılır.

```bash
cmm tui
```

```
 ┌────────────────────────────────────────────────────────────────────────────────────────┐
 │  Cloud Mod Manager (cmm)   Profile: Survival | MC: 1.21.1 | Loader: fabric             │
 ├────────────────────────────────────────────────────────────────────────────────────────┤
 │   [1. Installed Mods]      2. Modrinth Search      3. Config Editor      4. Sync       │
 ├────────────────────────────────────────────────────────────────────────────────────────┤
 │  STATUS      NAME                  SLUG                VERSION         SIDE            │
 │  [PIN]       Lithium               lithium             0.12.7          both            │
 │  [UPDATE]    Sodium                sodium              0.5.8           both            │
 │  [OK]        FerriteCore           ferrite-core        6.0.1           both            │
 │                                                                                        │
 ├────────────────────────────────────────────────────────────────────────────────────────┤
 │  [Tab/1-4] Sekme Değiştir  •  [?] Yardım  •  [p] Pin  •  [d] Sil  •  [u] Güncelle      │
 └────────────────────────────────────────────────────────────────────────────────────────┘
```

### TUI Klavye Kısayolları (Keybindings)

| Tuş | Eylem |
|---|---|
| `Tab` / `Shift+Tab` | Sekmeler arasında ileri / geri geçiş yapar. |
| `1`, `2`, `3`, `4` | Doğrudan ilgili sekmeye atlar (`Mods`, `Search`, `Config`, `Sync`). |
| `↑` / `↓` veya `k` / `j` | Listede veya form alanlarında yukarı/aşağı gezinir. |
| `Enter` | Mod detaylarını açar, arama sürümünü seçer veya form kaydeder. |
| `p` | Seçili modu sabitler / sabitlemeyi kaldırır (`[PIN]`). |
| `d` veya `x` | Seçili modu silmek için onay modalı açar. |
| `u` | Seçili mod için güncelleme kontrolü yapar. |
| `/` | Yüklü modlar listesinde anlık arama/filtreleme çubuğunu açar. |
| `i` | Arama sekmesinde seçili modu tek tuşla yükler. |
| `s` | Ayar düzenleyicide değişiklikleri `cmm.toml` dosyasına kaydeder. |
| `r` | Ayar düzenleyicide değişiklikleri iptal edip diskten yeniden yükler. |
| `?` veya `F1` | Kısayol yardım penceresini (Help Modal) açar/kapatır. |
| `Esc` | Açık modalleri veya arama filtresini kapatır. |
| `q` veya `Ctrl+C` | TUI arayüzünden temiz bir şekilde çıkar. |

---

## 5. Yapılandırma Dosyaları (`cmm.toml` & `cmm.lock`)

### `cmm.toml` (Kullanıcı Yapılandırması)
```toml
[profile]
name = "MySurvivalServer"
minecraft_version = "1.21.1"
loader = "fabric"
loader_version = "0.19.3"
side = "server" # "server", "client" veya "both"

[paths]
mods_dir = "mods"
config_dir = "config"

[modrinth]
# Opsiyonel: Yüksek hız sınırları için Modrinth API tokenı
token = ""

[server]
# cmm serve için ayarlar
port = 8080
sync_token = "my-secret-token"
```

### `cmm.lock` (Sürüm Kilit Dosyası)
`cmm.lock`, sunucunuzda kurulu modların değişmez (immutable) kopyasını tutar. `git` deposuna eklenerek tüm sunucuların birebir aynı modları ve hash'leri kullanması sağlanır:
```toml
[[mods]]
slug = "lithium"
name = "Lithium"
source = "modrinth"
project_id = "gvQqBUqZ"
version_id = "VvH3p7F4"
version = "0.12.7"
file_name = "lithium-fabric-mc1.21.1-0.12.7.jar"
sha512 = "a6f8b..."
download_url = "https://cdn.modrinth.com/..."
side = "both"
pinned = true
```

---

## 6. Gerçek Hayat Senaryoları & İpuçları

### Senaryo 1: GitHub CI/CD ile Otomatik Sunucu Dağıtımı
1. Geliştirme makinenizde modları kurun ve sabitleyin:
   ```bash
   cmm add lithium
   cmm pin lithium
   git add cmm.toml cmm.lock
   git commit -m "Add lithium mod"
   git push origin main
   ```
2. Oracle Cloud veya üretim sunucunuzda tek komutla eşitleyin:
   ```bash
   cmm sync --source github --repo my-org/my-server --branch main
   ```

### Senaryo 2: Var Olan Bir Sunucuyu `cmm` Yönetimine Alma
Sunucunuzda zaten onlarca `.jar` dosyası varsa, tek tek mod aramanıza gerek yoktur:
```bash
cd /opt/minecraft/server
cmm init --mc-version "1.21.1" --loader "fabric" --side "server"
cmm sync local
```
`cmm` tüm JAR'ların sağlama toplamlarını Modrinth ile eşleştirir, kilit dosyasını üretir ve tanınmayan özel JAR'lar varsa sizi uyarır.

---

## 🎯 Destek & İpuçları
- **Yardım**: Herhangi bir komut için `--help` ekleyerek (örn. `cmm sync --help`) tüm bayrakları görebilirsiniz.
- **Oracle Cloud Performansı**: `cmm-linux-arm64` ikilisi ARM64 Ampere işlemciler için sıfır CGO bağımlılığı ile saf Go'da derlenmiştir; RAM ve CPU tüketimi asgari düzeydedir (<15MB RAM).
