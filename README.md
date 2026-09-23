# ⚡ EKT.KZ — AI Ассистент (ГК Электрокомплект)

AI-консультант для интернет-магазина электрооборудования **ekt.kz** на Go + OpenAI GPT-4o-mini.

## 🚀 Быстрый старт

```bash
git clone https://github.com/BAITC-Hacks/hack-400d70df-donerka-house
cd hack-400d70df-donerka-house
go mod tidy

# С AI (нужен OpenAI API Key)
OPENAI_API_KEY=sk-... go run main.go

# С официальным API EKT (Basic Auth; секреты задаются только на сервере)
EKT_API_BASE_URL=https://ekt.kz/api \
EKT_API_USERNAME=apiuser \
EKT_API_PASSWORD=... \
OPENAI_API_KEY=sk-... go run main.go

# Демо-режим (без ключа)
go run main.go
```

Открыть: **http://localhost:8080**

---

## 📁 Структура проекта

```
├── main.go              # HTTP сервер, загрузка каталога
├── handlers/
│   ├── chat.go          # POST /api/chat — OpenAI с контекстом каталога
│   └── menu.go          # GET  /api/catalog — список товаров
├── models/
│   └── types.go         # Go-структуры данных (Product, ProductDetail, etc.)
├── data/
│   ├── products.json    # Список товаров (страница 2) — формат API ekt.kz
│   ├── products2.json   # Список товаров (страница 1)
│   └── detail.json      # Детальная информация о товаре
├── static/
│   ├── index.html       # Сайт в стиле ekt.kz (синий/красный)
│   ├── style.css        # Стили
│   └── chat.js          # AI чат-виджет
├── Dockerfile
└── README.md
```

---

## 🌐 API

| Метод | Endpoint | Описание |
|-------|----------|----------|
| `GET` | `/` | Главная страница |
| `GET` | `/api/catalog` | Все товары из JSON файлов |
| `POST` | `/api/chat` | Вопрос AI-ассистенту; ищет по live-каталогу EKT и при выборе добавляет товар |
| `GET` | `/api/cart` | Получить корзину текущей браузерной сессии |
| `POST` | `/api/cart` | Добавить товар: `{ "article": "200300285_", "quantity": 1 }` |
| `DELETE` | `/api/cart?article=200300285_` | Удалить товар; без `article` очистить корзину |
| `GET` | `/health` | Статус сервера |

### Пример чата:
```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Нужен автоматический выключатель 160А, что есть?"}'
```

### Пример корзины:
```bash
# Cookie из ответа сохраняет корзину текущего клиента
curl -c cookies.txt -b cookies.txt http://localhost:8080/api/cart
curl -c cookies.txt -b cookies.txt -X POST http://localhost:8080/api/cart \
  -H "Content-Type: application/json" \
  -d '{"article":"200300285_","quantity":1}'
curl -c cookies.txt -b cookies.txt -X DELETE \
  'http://localhost:8080/api/cart?article=200300285_'
```

---

## 📦 Формат JSON файлов

Файлы совместимы с API ekt.kz (`/api/products/` и `/api/products/detail`):

**products.json / products2.json:**
```json
{
  "page": 1, "per_page": 20, "count": 20,
  "items": [
    { "id": 515291, "name": "...", "article": "...", "price": 64920, "image": "...", "url": "...", "url_api_detail": "..." }
  ]
}
```

**detail.json:**
```json
{
  "id": 515291, "name": "...", "article": "...",
  "description": "...", "price": 64920, "quantity": 23,
  "stores": [{"id": 3, "name": "Шымкент", "quantity": 2}],
  "properties": { ... }
}
```

---

## ➕ Добавить больше товаров

Просто положи дополнительные файлы JSON в папку `data/`:
- `products3.json`, `products4.json` и т.д.
- Обнови `main.go` → массив `files` чтобы подключить их

## 🛒 Корзина и AI-добавление

Корзина привязана к HttpOnly-cookie `ekt_session` и хранится в памяти процесса. Это подходит для демо и одного экземпляра приложения. Перед production-развертыванием замени `handlers.CartStore` на Redis или базу данных, чтобы корзины не терялись после перезапуска и работали между несколькими экземплярами.

Сервер проверяет артикул товара и количество от 1 до 999. Явные команды `купить`, `купи`, `беру`, `добавь`, `закажи` и `оформляю` сразу обрабатываются на сервере: товар добавляется в корзину без дополнительного вопроса и без ожидания подтверждения. Если товар не удаётся однозначно определить, ассистент попросит указать артикул или выбрать карточку.

## 🔎 Live-каталог EKT

Перед ответом ассистент ищет товары на публичной странице `https://nursultan.ekt.kz/catalog/?q=...` и загружает страницы найденных товаров, чтобы получить описание, назначение и характеристики. JSON API EKT требует отдельную авторизацию, поэтому приложение не подменяет её OpenAI-ключом и не отправляет его на EKT. Региональный адрес можно изменить через `EKT_CATALOG_URL`.

При заданных `EKT_API_USERNAME` и `EKT_API_PASSWORD` сервер использует официальный API EKT с Basic Auth: загружает страницы `/api/products` и получает детали выбранных товаров через `/api/products/detail?id=...`. Эти секреты передаются только серверу через `.env`/Docker Compose и не хранятся в репозитории. Если API временно недоступен или переменные не заданы, остаётся безопасный fallback на локальный снимок и публичные HTML-страницы.

Из карточки EKT читается актуальный статус наличия текущего региона: `В наличии` или `Под заказ`. При доступном API ассистент дополнительно получает общий остаток и разбивку по складам. Поэтому он может ответить, например: «В наличии в Астане, до 8 шт.; общий остаток по EKT — 23 шт.» или «Под заказ».

Ассистент сохраняет короткий контекст браузерной сессии: после показа карточек можно спросить «для чего нужен этот товар?» или сказать «беру первый», и он использует сведения именно из выбранных карточек. Повторное подтверждение после явного выбора не требуется.

---

## 🐳 Docker

```bash
docker build -t ekt-ai .
docker run -p 8080:8080 -e OPENAI_API_KEY=sk-... ekt-ai
```

---

## ☁️ Деплой на Railway

1. [railway.app](https://railway.app) → "New Project" → "Deploy from GitHub"
2. Выбери: `BAITC-Hacks/hack-400d70df-donerka-house`
3. Variables: `OPENAI_API_KEY=sk-...`
4. Готово — Railway соберёт через Dockerfile автоматически

---

## 🔑 OpenAI API Key

[platform.openai.com/api-keys](https://platform.openai.com/api-keys)
Модель `gpt-4o-mini` — ~\$0.0002 за запрос

---

## 🛠️ Стек

- **Backend**: Go 1.21 (`net/http`)
- **AI**: OpenAI GPT-4o-mini
- **Data**: JSON файлы из API ekt.kz
- **Frontend**: HTML/CSS/JS (без фреймворков, стиль ekt.kz)
- **Deploy**: Docker + Railway/Render
