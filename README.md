# ⚡ EKT.KZ — AI Ассистент (ГК Электрокомплект)

AI-консультант для интернет-магазина электрооборудования **ekt.kz** на Go + OpenAI GPT-4o-mini.

## 🚀 Быстрый старт

```bash
git clone https://github.com/BAITC-Hacks/hack-400d70df-donerka-house
cd hack-400d70df-donerka-house
go mod tidy

# С AI (нужен OpenAI API Key)
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
| `POST` | `/api/chat` | Вопрос AI-ассистенту |
| `GET` | `/health` | Статус сервера |

### Пример чата:
```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Нужен автоматический выключатель 160А, что есть?"}'
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
