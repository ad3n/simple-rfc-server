# RFC mock untuk Lawang-Core

Mock RFC native dengan definisi function, parameter, dan output di YAML. Server berjalan melalui **WebSocket RFC** pada port `8088`, tanpa SAP Gateway atau SAP ABAP backend.

Client penguji menggunakan [`github.com/ad3n/gonwrfc`](https://github.com/ad3n/gonwrfc) `v1.0.11`, sama dengan dependency Lawang-Core lokal. Karena library ini hanya menyediakan API client, implementasi server memakai binding cgo kecil ke SAP NetWeaver RFC SDK. Server juga menyediakan `RFC_GET_FUNCTION_INTERFACE` agar client dapat mengambil metadata dari YAML.

## Prasyarat

- Go 1.27.3 atau lebih baru dan C compiler.
- SAP NetWeaver RFC SDK dengan dukungan WebSocket RFC, sesuai OS/arsitektur. Pengujian lokal memakai SDK 7.50 PL18 macOS arm64.
- SDK dipasang sendiri; library SAP tidak disertakan di repository.

Server menerima semua kredensial dan menonaktifkan TLS untuk testing. SDK membuka port jaringan; jalankan hanya pada mesin/jaringan development yang terisolasi. Ini bukan server production.

## Menjalankan

```sh
make run SAPNWRFC_HOME="$HOME/nwrfcsdk"
```

`SAPNWRFC_HOME` harus berisi direktori `include` dan `lib`. Default Makefile adalah `/usr/local/sap`. Di mesin ini SDK yang berhasil diuji berada di `$HOME/nwrfcsdk`.

Untuk build dan menjalankan konfigurasi lain:

```sh
make build SAPNWRFC_HOME="$HOME/nwrfcsdk"
export DYLD_LIBRARY_PATH="$HOME/nwrfcsdk/lib" # macOS
# export LD_LIBRARY_PATH="$HOME/nwrfcsdk/lib" # Linux
./bin/rfc-server -config mock.yml -check
./bin/rfc-server -config mock.yml
```

Hentikan dengan Ctrl+C. Restart server setelah mengubah YAML. Client yang menyimpan metadata juga perlu reconnect/restart ketika nama atau tipe parameter berubah.

## Konfigurasi YAML

Lihat [mock.yml](mock.yml) untuk contoh lengkap:

```yaml
port: 8088
functions:
  - name: Z_LAWANG_TEST
    parameters:
      - name: IV_NAME
        direction: import
        type: STRING
      - name: EV_NAME
        direction: export
        type: STRING
        from: IV_NAME
      - name: EV_STATUS
        direction: export
        type: STRING
        value: SUCCESS
```

- `name`: nama function/parameter uppercase, maksimal 30 karakter (`A-Z`, angka, underscore; harus diawali huruf).
- `direction`: `import`, `export`, atau `changing`.
- `type`: `CHAR`, `STRING`, `INT` (signed 32-bit), atau `FLOAT`.
- `length`: wajib untuk `CHAR`, dalam unit UTF-16, antara 1–65535.
- `value`: output statis; tipe YAML harus sesuai. Beri tanda kutip untuk string angka.
- `from`: nama parameter `import`/`changing` yang disalin ke output. Tipe dan panjang harus sama. Nilai diambil sebelum output lain ditulis.
- Output tanpa `value`/`from` menggunakan nilai awal SDK; `changing` mempertahankan input.

`value` dan `from` tidak boleh dipakai bersamaan. Field YAML yang tidak dikenal, nama duplikat, referensi tidak valid, dan nilai di luar rentang ditolak saat startup. `RFC_GET_FUNCTION_INTERFACE` disediakan internal dan tidak boleh didefinisikan ulang.

Versi awal ini belum mendukung `STRUCTURE`, `TABLE`, `DATE`, `BCD`, conditional response, atau skenario ABAP exception. Karena belum ada tabel, gunakan output scalar/map di Lawang-Core; alur RFC sync tabel belum tercakup.

## Menguji dengan gonwrfc

Dengan server tetap berjalan, buka terminal kedua:

```sh
export DYLD_LIBRARY_PATH="$HOME/nwrfcsdk/lib"
./bin/rfc-call
./bin/rfc-call -function Z_LAWANG_TEST -params '{"IV_NAME":"Lawang","CV_COUNTER":42}'
```

Respons contoh pertama:

```json
{
  "ECHOTEXT": "Hello Lawang",
  "RESPTEXT": "Hello from RFC mock Lawang-Core"
}
```

Client menerima `-host`, `-port`, `-function`, dan `-params` (JSON), serta exit nonzero jika panggilan gagal. Ini protokol RFC melalui WebSocket; bukan endpoint HTTP JSON yang dipanggil menggunakan curl.

## Koneksi Lawang-Core

Gunakan backend RFC dengan nilai berikut (gabungkan dengan field backend lain yang diperlukan endpoint Anda):

```json
{
  "name": "rfc-mock",
  "protocol": "rfc",
  "hosts": ["/?wshost=127.0.0.1&wsport=8088&use_tls=0&lang=EN"],
  "username": "MOCK",
  "password": "MOCK",
  "database": "000"
}
```

Lawang-Core `ParseConnectionParams` meneruskan query string ke gonwrfc dan menambahkan `ashost`/`sysnr`. URL lengkapnya menjadi `rfc:///?wshost=127.0.0.1&wsport=8088&use_tls=0&lang=EN`. Authority URL sengaja kosong sehingga `ashost` dan `sysnr` kosong; SDK kemudian memakai `wshost`/`wsport`. Jangan isi hostname URL seperti `rfc://127.0.0.1:00`, karena SDK akan memilih koneksi SAP klasik dan mencoba port 3300. Jika Lawang-Core berjalan di container, ganti nilai `wshost` dengan alamat host mock yang bisa diakses container (misalnya `host.docker.internal` pada Docker Desktop).

Pilih function `STFC_CONNECTION` dengan parameter `{"REQUTEXT":"Hello Lawang"}`, atau `Z_LAWANG_TEST` dengan `{"IV_NAME":"Lawang","CV_COUNTER":42}`. Untuk jalur JavaScript `Rfc`/`Service`, gunakan `Output` yang tidak nil dengan `Key` kosong agar hasil dikembalikan sebagai object, bukan tabel.

Integration test menggunakan parameter koneksi yang sama dengan hasil parsing Lawang-Core. Pengujian endpoint HTTP Lawang-Core secara penuh membutuhkan konfigurasi endpoint dan instance Lawang-Core yang sedang berjalan.

## Verifikasi

```sh
make check SAPNWRFC_HOME="$HOME/nwrfcsdk"
make test SAPNWRFC_HOME="$HOME/nwrfcsdk"
```

`make check` membutuhkan `betteralign` di PATH. Untuk integration test terhadap server terpisah yang menjalankan `mock.yml`:

```sh
RFC_MOCK_INTEGRATION=1 make test SAPNWRFC_HOME="$HOME/nwrfcsdk"
```

Gunakan `RFC_MOCK_PORT` jika port contoh diubah. Tanpa flag environment tersebut, integration test dilewati. Test mencakup ping, metadata, echo Unicode, batas CHAR, output YAML function custom, dan function tidak dikenal.
