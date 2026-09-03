# 📋 Cloud Mod Manager (`cmm`) — Detaylı Özellik Listesi ve Teknik Tasarım Planı (Feature List)

Bu doküman, Cloud Mod Manager (`cmm`) projesine eklenecek olan 8 yeni özelliğin mimari tasarımını, komut sözdizimlerini, kullanıcı deneyimi (UX) akışlarını, uç durum (edge-case) yönetimlerini ve teknik detaylarını kapsamaktadır.

Benzer ve birbiriyle ilişkili özellikler mantıksal olarak **3 ana grup altında** toplanmıştır.

---

## 📑 İçindekiler
1. [Grup 1: Gelişmiş Mod ve Sürüm Yönetimi (`cmm update` & `cmm install`)](#-grup-1-gelişmiş-mod-ve-sürüm-yönetimi-cmm-update--cmm-install)
   - [1.1. `cmm update` Geliştirmeleri (Slug Gösterimi, Kanal Filtresi, Çoklu Slug ve Toplu Onay)](#11-cmm-update-geliştirmeleri)
   - [1.2. `cmm install` / `cmm add` Sürüm Yönetimi ve Çoklu Kurulum](#12-cmm-install--cmm-add-sürüm-yönetimi-ve-çoklu-kurulum)
2. [Grup 2: Akıllı Sunucu Taraması ve Kesin Sürüm Tespiti (`cmm scan`)](#-grup-2-akıllı-sunucu-taraması-ve-kesin-sürüm-tespiti-cmm-scan)
   - [2.1. Çok Katmanlı Kesin Sürüm (Minecraft & Loader) Tespiti ve Onay Döngüsü](#21-çok-katmanlı-kesin-sürüm-minecraft--loader-tespiti-ve-onay-döngüsü)
   - [2.2. Tarama ve Güncelleme Sırasında Pasif Loader Bildirimi](#22-tarama-ve-güncelleme-sırasında-pasif-loader-bildirimi)
3. [Grup 3: Yükleyici (Loader) ve Temel Ortam Yönetimi (`cmm loader` & `cmm mc-version`)](#-grup-3-yükleyici-loader-ve-temel-ortam-yönetimi-cmm-loader--cmm-mc-version)
   - [3.1. `cmm loader update` Komutu ve Süreç Güvenlik Kilidi (Process Safety Check)](#31-cmm-loader-update-komutu-ve-süreç-güvenlik-kilidi)
   - [3.2. `cmm mc-version` Komutu (Görüntüleme, Doğrulama ve Güncelleme)](#32-cmm-mc-version-komutu)
4. [Teknik Mimari ve Veri Yapıları Değişiklikleri](#-teknik-mimari-ve-veri-yapıları-değişiklikleri)
5. [Uç Durumlar (Edge Cases) ve Hata Yönetimi](#-uç-durumlar-edge-cases-ve-hata-yönetimi)

---

## 🚀 Grup 1: Gelişmiş Mod ve Sürüm Yönetimi (`cmm update` & `cmm install`)

### 1.1. `cmm update` Geliştirmeleri

#### A. Slug Gösterimi
- **Mevcut Durum**: Çıktıda yalnızca mod adı görünmekteydi (`- Sodium: 0.5.8 -> 0.5.11`).
- **Yeni Tasarım**: Modun Modrinth üzerindeki benzersiz tanımlayıcısı olan `slug` bilgisi, adının hemen yanında parantez içinde açıkça yazılır.
- **Örnek Çıktı**:
  ```text
  - Sodium (sodium): 0.5.8 -> 0.5.11
  - Lithium (lithium): 0.11.2 -> 0.12.1
  - Iris Shaders (iris): 1.7.0 -> 1.7.5
  ```

#### B. Sürüm Kanalı Filtresi (`--channel`)
- **Parametre**: `--channel <release|beta|alpha>` (Kısa yol: `-c`)
- **Varsayılan Değer**: `release`
- **Kanal Mantığı**:
  - `release`: Sadece kararlı (Release) sürümleri kabul eder.
  - `beta`: Hem kararlı hem de Beta sürümleri kabul eder (`release` + `beta`).
  - `alpha`: Tüm kararlılık seviyelerini kabul eder (`release` + `beta` + `alpha`).
- **Başlık Gösterimi**: Komut çalıştırıldığında çıktının en başında hangi kanalın aktif olduğu açıkça belirtilir.
- **Örnek Çıktı**:
  ```text
  🔍 Checking for updates... [Channel: release]
  Found 2 available update(s):

  - Sodium (sodium): 0.5.8 -> 0.5.11
  - Iris Shaders (iris): 1.7.0 -> 1.7.5

  Apply updates? [y/N]: 
  ```

#### C. Çoklu Slug Desteği ve Toplu Onay
- **Kullanım**: `cmm update [slug1] [slug2] [slug3] ...`
  - Örn: `cmm update sodium iris lithium`
- **İşleyiş**:
  - Eğer argüman verilmezse (`cmm update`), kurulu tüm modlar taranır.
  - Birden fazla slug verilirse, yalnızca bu modlar için hedef kanaldaki güncellemeler kontrol edilir.
  - Kurulu olmayan veya bulunamayan bir slug girilirse kullanıcı bilgilendirilir.
  - İndirme işlemi başlamadan önce güncellenecek tüm modlar listelenir ve tek bir toplu onay (`Apply updates? [y/N]: `) istenir. Onay verilmezse hiçbir dosya değiştirilmez.

---

### 1.2. `cmm install` / `cmm add` Sürüm Yönetimi ve Çoklu Kurulum

`cmm add` ve `cmm install` komutları birbirinin tam alias'ı (eşdeğeri) olacak şekilde birleştirilir.

#### A. Kanal Filtresi (`--channel`)
- Mod kurulumunda da `--channel release|beta|alpha` bayrağı desteklenir (varsayılan: `release`).

#### B. Tek Mod & Sürüm Belirtilmemişse (İnteraktif Sayfalama)
- **Komut**: `cmm install sodium` (veya `cmm add sodium`)
- **İşleyiş**:
  - Seçilen kanaldaki (`release`) son 10 sürüm numaralandırılarak tablo şeklinde sunulur.
  - Kullanıcı sürüm numarasını seçebilir, `n` (sonraki sayfa) veya `p` (önceki sayfa) ile gezinebilir.
- **Örnek Terminal Akışı**:
  ```text
  🔍 Available versions for Sodium (sodium) [Channel: release, Loader: fabric, MC: 1.21.1]:

    [1] 0.5.11  (mc1.21.1) - sodium-fabric-0.5.11.jar (Published: 2024-08-15)
    [2] 0.5.10  (mc1.21.1) - sodium-fabric-0.5.10.jar (Published: 2024-07-28)
    [3] 0.5.9   (mc1.21.1) - sodium-fabric-0.5.9.jar  (Published: 2024-07-02)
    [4] 0.5.8   (mc1.21.1) - sodium-fabric-0.5.8.jar  (Published: 2024-06-14)
    ...
    [10] 0.5.0  (mc1.21.1) - sodium-fabric-0.5.0.jar  (Published: 2024-04-10)

  Page 1/3 (Showing 1-10 of 28). [n] Next page, [q] Cancel
  Select version [1-10]: 1

  Download and install sodium-fabric-0.5.11.jar? [Y/n]: y
  ⬇️  Downloading sodium-fabric-0.5.11.jar...
  ✅ Successfully installed Sodium (0.5.11)
  ```

#### C. Tek Mod & Belirli Sürüm
- **Komut**: `cmm install sodium -v "0.5.11"`
- **İşleyiş**: İnteraktif menü açılmadan doğrudan belirtilen sürüm hedeflenir. İndirme öncesinde dosya adı onaylanır:
  ```text
  Target: Sodium (sodium) version 0.5.11 (File: sodium-fabric-0.5.11.jar)
  Install this version? [Y/n]: y
  ```

#### D. Çoklu Mod Kurulumu (`cmm install slug1 slug2 slug3`)
- **Komut**: `cmm install sodium iris lithium`
- **İşleyiş**:
  - Terminal akışının bozulmaması için **interaktif sürüm seçimi otomatik olarak devre dışı kalır**.
  - Seçilen kanaldaki (`release`) en son kararlı sürümler Modrinth üzerinden çözümlenir.
  - Toplu özet tablosu basılarak tek bir son onay istenir:
  ```text
  📦 Preparing to install 3 mod(s) [Channel: release]:
  - Sodium (sodium) -> 0.5.11 (sodium-fabric-0.5.11.jar)
  - Iris Shaders (iris) -> 1.7.5 (iris-fabric-1.7.5.jar)
  - Lithium (lithium) -> 0.12.1 (lithium-fabric-0.12.1.jar)

  Proceed with installation? [Y/n]: y
  ```

#### E. Zaten Kurulu Mod Uyarısı ve Değiştirme (Replacement) Koruması
- Eğer kurulmak istenen mod zaten `cmm.lock` içinde mevcutsa:
  ```text
  ⚠️  Mod 'Sodium' is already installed (Current: 0.5.8, Target: 0.5.11).
  Replace installed version with 0.5.11? [y/N]: y
  ```
- Kullanıcı onaylarsa:
  1. Eski JAR dosyası diskten (`mods/sodium-fabric-0.5.8.jar`) silinir.
  2. Yeni JAR dosyası indirilir ve doğrulanır.
  3. `cmm.lock` kaydı yeni sürüm, hash ve dosya adıyla güncellenir.

---

## 🔍 Grup 2: Akıllı Sunucu Taraması ve Kesin Sürüm Tespiti (`cmm scan`)

### 2.1. Çok Katmanlı Kesin Sürüm (Minecraft & Loader) Tespiti ve Onay Döngüsü

Mevcut tarama algoritması, tahmin hatalarını önlemek amacıyla **3 aşamalı hiyerarşik doğrulama** modeline geçirilir:

#### A. 1. Aşama: Deterministik (Kesin) Kaynaklar
1. **Server JAR İçeriği**: `server.jar` veya kökteki sunucu JAR dosyası ZIP olarak açılıp içindeki `version.json` okunur (`"id": "1.21.1"` veya `"id": "26.2"`).
2. **Fabric / Quilt Özellik Dosyaları**: `fabric-server-launcher.properties` (içindeki `serverJar=` hedefi) veya `.fabric-installer`.
3. **İstemci Profil Manifestoları**: `instance.json` (Prism/MultiMC), `minecraft.json` veya `modrinth.index.json`.

#### B. 2. Aşama: Heuristic (Tahmin) ve Kullanıcı Etkileşim Döngüsü
Kesin bir dosya bulunamazsa (yalnızca modların olduğu bir klasör tarandığında):
1. Mod dosyalarından tespit edilen en popüler sürüm kullanıcıya önerilir:
   ```text
   Could not determine exact Minecraft version. Use detected version (26.2)? [Y/n]: 
   ```
2. Eğer kullanıcı `n` veya `N` derse, serbest giriş istenir:
   ```text
   Enter Minecraft version: 1.21.1
   ```
3. Kullanıcının girdiği sürüm doğrulanır (`^[0-9]+(\.[0-9]+)+.*$`) ve `cmm.toml` dosyasına yazılır.

#### C. Loader Sürümü İçin Aynı Onay Mantığı
Loader tipi (Fabric, Forge, NeoForge, Quilt) ve sürümü için de aynı kesin dosya kontrolü yapılır; kesin bulunamazsa kullanıcıya onay sorulur:
```text
Could not determine exact Loader version. Use detected version (0.16.10)? [Y/n]: 
```

---

### 2.2. Tarama ve Güncelleme Sırasında Pasif Loader Bildirimi

`cmm update` veya `cmm scan` çalıştırıldığında, ana işlem akışı kesintiye uğramadan arka planda mevcut loader'ın yeni bir versiyonu olup olmadığı sorgulanır.

- **Kural**: Çıktının ortasını bölmez; komutun tüm çıktıları bittikten sonra en son satırda tek satırlık bir bilgi notu olarak gösterilir.
- **Örnek Çıktı**:
  ```text
  ...
  Successfully updated 3 mods.

  Notice: A new Fabric Loader version is available (v0.19.5). Run: cmm loader update
  ```

---

## ⚙️ Grup 3: Yükleyici (Loader) ve Temel Ortam Yönetimi (`cmm loader` & `cmm mc-version`)

### 3.1. `cmm loader update` Komutu ve Süreç Güvenlik Kilidi

#### A. Komut Davranışı
- `cmm loader update`: Yapılandırılmış olan loader türü (örn: Fabric) için en güncel stabil sürümü Modrinth/Fabric meta API üzerinden kontrol eder.
- Güncelleme varsa kullanıcıdan onay ister:
  ```text
  Current Fabric Loader: v0.16.9
  Latest Fabric Loader:  v0.16.10
  Update Fabric Loader to v0.16.10? [y/N]: y
  ```

#### B. Süreç Güvenlik Kontrolü (Process Safety Check)
Açık ve çalışan bir Minecraft sunucusu veya istemcisi varken loader dosyalarını değiştirmek dosya bozulmasına ve kilitlenmelere yol açabilir.
- **Kontrol Mekanizması**:
  - Linux/macOS üzerinde: `/proc` tablosu taranarak veya `pgrep`/süreç adı kontrolüyle aktif `java` / `minecraft` süreçleri aranır (özel olarak sunucu dizininde çalışan jar dosyaları kontrol edilir).
  - Windows üzerinde: `tasklist` veya `wmic process` ile eşleşen Java süreçleri tespit edilir.
- **Hata Durumu**:
  Eğer aktif bir süreç tespit edilirse işlem anında durdurulur:
  ```text
  Error: Server is currently running. Please stop the server before updating the loader (or use --force).
  ```
- **Bypass**: `--force` bayrağı verilirse kontrol atlanır.

---

### 3.2. `cmm mc-version` Komutu

Minecraft sürümünü görüntülemek veya güvenle değiştirmek için özel CLI komutu.

#### A. Argümansız Kullanım (Görüntüleme)
```bash
cmm mc-version
```
- **Çıktı**:
  ```text
  Configured Minecraft version: 26.2
  ```

#### B. Argümanlı Kullanım (Değiştirme)
```bash
cmm mc-version "26.1.2"
```
- **Format Doğrulama**: Girilen değerin geçerli bir Minecraft versiyon formatında olup olmadığı (`regexp: ^(1\.[0-9]+(\.[0-9]+)?|2[0-9]\.[0-9]+|[0-9]{2}w[0-9]{2}[a-z])$`) kontrol edilir. Geçersiz bir metin girilirse hata döndürülür (`Error: invalid Minecraft version format 'xyz'`).
- **Onay İsteme**:
  ```text
  Change configured Minecraft version from 26.2 to 26.1.2? [y/N]: y
  Minecraft version updated to 26.1.2 in cmm.toml.
  ```
- **Sonuç**: `cmm.toml` dosyası temiz bir şekilde güncellenir. Kullanıcıya kilit dosyasını senkronize etmesi için `cmm update` veya `cmm sync` çalıştırması hatırlatılır.

---

## 🏗️ Teknik Mimari ve Veri Yapıları Değişiklikleri

### 1. `internal/modrinth/client.go` & `types.go`
- `ReleaseChannel` tipi eklenir:
  ```go
  type ReleaseChannel string
  const (
      ChannelRelease ReleaseChannel = "release"
      ChannelBeta    ReleaseChannel = "beta"
      ChannelAlpha   ReleaseChannel = "alpha"
  )
  ```
- `GetProjectVersions` metoduna `channel ReleaseChannel` desteği eklenir. `release` istendiğinde yalnızca `version_type == "release"` olanlar; `beta` istendiğinde `release` ve `beta`; `alpha` istendiğinde hepsi filtrelenir.

### 2. `internal/mod/manager.go`
- `CheckUpdatesMulti(slugs []string, channel string, force bool) ([]UpdateCandidate, []string, error)` fonksiyonu eklenir.
- `InstallModVersion(slug string, versionID string, channel string) (*AddResult, error)` fonksiyonu eklenir.
- `DetectProcessRunning(baseDir string) (bool, error)` fonksiyonu eklenir (Cross-platform OS süreç taraması).

### 3. `internal/loader/manager.go`
- `CheckLatestLoaderVersion(loaderName string, gameVersion string) (latestVer string, updateAvailable bool, err error)` fonksiyonu eklenir.
- `UpdateLoader(loaderName string, newVersion string) error` fonksiyonu eklenir.

---

## 🛡️ Uç Durumlar (Edge Cases) ve Hata Yönetimi

| Uç Durum (Edge Case) | Beklenen Sistem Davranışı |
| :--- | :--- |
| **Seçili kanalda hiç sürüm olmaması** (Örn: Mod sadece Alpha yayınlamış ama kanal `--channel release`) | Mod atlanır ve `"No compatible release versions found for mod 'X' (Try --channel beta or --channel alpha)"` uyarısı verilir. |
| **İnteraktif kurulumda geçersiz numara seçimi** | Hata basılıp tekrar giriş istenir; `q` veya `Ctrl+C` ile güvenle iptal edilebilir. |
| **Çoklu kurulumda modlardan birinin Modrinth'te bulunamaması** | Bulunamayan mod hata olarak listelenir, diğer modların kurulum onay tablosu bozulmaz; kullanıcıya kalanları kurup kurmak istemediği sorulur. |
| **Sunucu çalışırken `loader update` çalıştırılması** | Süreç tespiti tetiklenir, işlem durdurulur ve `--force` önerilir. |
| **`cmm mc-version` ile geçersiz sürüm girilmesi** | Regex doğrulamasından geçemez; dosya değiştirilmeden açıklayıcı format hatası döndürülür. |
| **Non-interactive / CI/CD Ortamları (stdin kapalıysa)** | Onay isteyen (`[y/N]`) yerlerde `--yes` / `-y` bayrağı desteklenir; bayrak verilmemişse varsayılan değer (`N`) kabul edilir ve işlem güvenle sonlandırılır. |

---

## 📌 Sıradaki Adım
Bu plan dokümanı, kullanıcı onayının ardından uygulama evresine geçildiğinde referans mimari olarak kullanılacaktır.
