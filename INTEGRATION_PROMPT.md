# Stashly — integratsiya prompti

Quyidagi matnni boshqa loyihada (backend yoki mobil ilova) fayl saqlash xizmatiga ulanish kerak bo'lganda AI yordamchiga yoki dasturchiga to'g'ridan-to'g'ri berish mumkin. `BASE_URL`, `USERNAME`, `PASSWORD` qiymatlarini o'zingiznikiga almashtiring.

---

## Prompt

Sen `Stashly` nomli fayl saqlash xizmatiga ulanadigan klient kodini yozasan. Xizmat REST API, JSON qaytaradi, vaqtlar RFC3339 (UTC) formatida.

### Ulanish ma'lumotlari

- Base URL: `BASE_URL` (masalan `https://storage.example.uz`)
- Klient hisobi: `USERNAME` / `PASSWORD` (admin panel orqali beriladi)

### Har bir so'rovga majburiy headerlar

| Header | Qiymat | Izoh |
|---|---|---|
| `X-Username` | `USERNAME` | klient logini |
| `X-Password` | `PASSWORD` | klient paroli |
| `X-Language` | `uz`, `ru` yoki `en` | majburiy |
| `X-Device` | qurilma/instansiya identifikatori | majburiy, har qanday satr (masalan server hostname yoki mobil qurilma UUID) |
| `X-Platform` | `android`, `ios`, `web` | ixtiyoriy, bo'lmasa `unknown` |
| `X-App-Version` | `1.2.3` | ixtiyoriy |

Header yetishmasa 401, parol noto'g'ri bo'lsa 401, klient bloklangan yoki nofaol bo'lsa 403 qaytadi.

### Xato formati (barcha endpointlar)

```json
{ "message": "sabab", "errors": { "maydon": ["xabar"] } }
```
`errors` faqat validatsiya (422) va fayl bo'yicha rad etishlarda bo'ladi. Statuslar: 401 auth, 403 bloklangan, 404 topilmadi (boshqa klientning fayli ham 404), 413 hajm yoki kvota, 415 MIME ruxsat etilmagan, 422 validatsiya, 429 limit.

### Paginatsiya formati

```json
{ "data": [...], "meta": { "page": 1, "per_page": 20, "total": 123, "last_page": 7 } }
```
Query: `page`, `per_page` (maksimum 100), `search`.

### File obyekti

```json
{
  "id": 1,
  "client_id": 3,
  "folder": "avatars",
  "name": "b3f1c2...9a.pdf",
  "original_name": "hisobot.pdf",
  "path": "3/avatars/2026/09/b3f1c2...9a.pdf",
  "url": "BASE_URL/storage/3/avatars/2026/09/b3f1c2...9a.pdf",
  "mime": "application/pdf",
  "extension": "pdf",
  "size": 123456,
  "sha256": "…64 hex…",
  "visibility": "public",
  "created_at": "2026-09-12T18:00:00Z"
}
```
`url` faqat `public` fayllarda bo'ladi, `private` fayllar uchun `null`. Public fayl `url` orqali headerlarsiz ochiladi. Private faylni faqat `/api/files/{id}/download` orqali olish mumkin. HTML, SVG, XML fayllar xavfsizlik uchun `attachment` sifatida beriladi (brauzerda ochilmaydi, yuklab olinadi).

Fayl nomi serverda har doim UUID ga almashtiriladi; asl nom `original_name` da saqlanadi. `id` ni o'z bazangizda saqlang, keyingi amallar (o'chirish, yuklab olish) `id` orqali.

### Endpointlar

#### 1. Fayl yuklash — `POST /api/files` (multipart/form-data)

Maydonlar:
- `file` — bitta fayl, yoki `files[]` — bir nechta fayl (bir so'rovda maksimum 20 ta)
- `folder` — ixtiyoriy. Faqat `a-z 0-9 _ -` va `/`, maksimum 3 daraja (`docs/2026/09`). Bo'sh bo'lsa ildiz.
- `visibility` — `public` (default) yoki `private`

Javob: `201`, har doim massiv:
```json
{ "data": [ File, File ] }
```

Qoidalar:
- Bitta fayl hajmi klient limiti (yoki server default 50 MB) dan oshsa 413 `file too large`.
- Klient kvotasi to'lsa 413 `quota exceeded`.
- Klientga faqat ma'lum MIME turlari ruxsat etilgan bo'lsa (masalan `image/*`), boshqa tur 415 `mime not allowed`. MIME fayl mazmunidan aniqlanadi, kengaytma yoki `Content-Type` ga ishonilmaydi.
- Bir nechta fayl yuborilganda hammasi avval tekshiriladi, bittasi rad etilsa butun so'rov rad etiladi va qaysi fayl ekani ko'rsatiladi:
  ```json
  { "message": "mime not allowed", "errors": { "files.2": ["mime not allowed"] } }
  ```
- Limit: klient uchun daqiqasiga 60 ta upload, oshsa 429.

curl:
```bash
curl -X POST "BASE_URL/api/files" \
  -H "X-Username: USERNAME" -H "X-Password: PASSWORD" \
  -H "X-Language: uz" -H "X-Device: backend-1" \
  -F "files[]=@/path/a.pdf" -F "files[]=@/path/b.png" \
  -F "folder=docs/2026" -F "visibility=private"
```

#### 2. Fayllar ro'yxati — `GET /api/files`
Query: `folder` (aniq papka), `search` (asl nom bo'yicha), `page`, `per_page`. Paginatsiya formatida `File` massivi.

#### 3. Bitta fayl — `GET /api/files/{id}`
`{ "data": File }`

#### 4. Faylni yuklab olish — `GET /api/files/{id}/download`
Binary, `Content-Disposition: attachment; filename="asl_nom"`. Public va private ikkalasi uchun ishlaydi.

#### 5. Faylni o'chirish — `DELETE /api/files/{id}`
`204`. Fayl diskdan va bazadan o'chadi, kvota bo'shaydi.

#### 6. Papkalar — `GET /api/folders`
Query: `parent` (ixtiyoriy, masalan `docs`). Javob:
```json
{ "data": [ { "name": "2026", "path": "docs/2026", "files_count": 12, "size": 12345 } ] }
```

#### 7. Klient ma'lumoti — `GET /api/me`
```json
{ "data": { "id": 3, "name": "...", "username": "...", "status": "active",
  "quota_bytes": 10737418240, "used_bytes": 123456, "files_count": 321,
  "allowed_mimes": ["image/*"], "max_file_size": 52428800 } }
```
`quota_bytes` yoki `max_file_size` `null` bo'lsa cheklov yo'q (server defaulti). Yuklashdan oldin `used_bytes + hajm <= quota_bytes` ni tekshirish tavsiya etiladi.

#### 8. Papkalarni ZIP qilish — `POST /api/archives` (JSON)

```json
{ "folders": ["docs/2026", "avatars"], "callback_url": "https://sizning.xizmat/hook" }
```
- `folders`: `[]` yoki `["*"]` — klientning barcha fayllari.
- `callback_url` ixtiyoriy. Faqat `http`/`https`, port 80/443, ommaviy IP (localhost, private tarmoq rad etiladi). Arxiv tayyor yoki xato bo'lganda shu manzilga `POST` bilan Archive JSON yuboriladi (10 s timeout, 3 urinish).

Javob: `202`, ish navbatga qo'yiladi va fonda bajariladi:
```json
{ "data": {
  "id": 7, "client_id": 3, "folders": ["docs/2026"],
  "status": "pending", "progress": 0, "files_count": 0,
  "size": null, "path": null, "url": null, "error": null,
  "callback_url": null, "expires_at": null,
  "created_at": "…", "finished_at": null } }
```
`status`: `pending` → `processing` → `done` | `failed` | `expired`.
`done` bo'lganda `url` (`BASE_URL/api/archives/7/download`), `size`, `files_count`, `expires_at` to'ladi. `expires_at` = `created_at` + 5 kun; shu muddatdan keyin zip avtomatik o'chadi va status `expired` bo'ladi.

Holatni bilish: `callback_url` bersangiz kutasiz, bermasangiz `GET /api/archives/{id}` ni 5 sekundda bir so'rab turasiz (`status`, `progress` 0..100).

#### 9. Arxivlar — `GET /api/archives` (paginatsiya), `GET /api/archives/{id}`

#### 10. ZIP ni yuklab olish — `GET /api/archives/{id}/download`
Binary zip, majburiy headerlar bilan. `done` bo'lmasa yoki muddati o'tgan bo'lsa 404. Zip ichida fayllar `papka/asl_nom` ko'rinishida.

#### 11. Arxivni o'chirish — `DELETE /api/archives/{id}` → `204`

### Tavsiya etiladigan integratsiya

1. Konfiguratsiyaga `STASHLY_BASE_URL`, `STASHLY_USERNAME`, `STASHLY_PASSWORD`, `STASHLY_DEVICE` qo'sh.
2. Bitta HTTP klient/servis yoz: headerlarni avtomatik qo'shadi, xatoni `{message, errors}` dan o'qiydi, 401/403 ni alohida loglaydi.
3. Yuklashdan keyin qaytgan `id`, `path`, `url`, `original_name`, `size`, `mime`, `sha256` ni o'z jadvalingga saqla. Foydalanuvchiga public fayl uchun `url` ni, private fayl uchun o'z backend'ing orqali `/api/files/{id}/download` ni proxy qilib ber (klient parolini frontendga chiqarma).
4. O'chirishda avval `DELETE /api/files/{id}`, keyin o'z yozuvingni o'chir. 404 kelsa faylni allaqachon yo'q deb hisobla.
5. Katta eksportlar uchun `POST /api/archives` + `callback_url` ishlat; zip 5 kun saqlanadi, kerak bo'lsa darhol yuklab ol.
6. `X-Password` ni hech qachon loglama va brauzerga bermа.
