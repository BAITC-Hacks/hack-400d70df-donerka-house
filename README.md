# 🌯 Donerka House — AI Chatbot Website

Сайт ресторана быстрого питания "Donerka House" с встроенным AI-ассистентом на Go + OpenAI GPT.

## 🚀 Быстрый старт

### 1. Установи зависимости

```bash
go mod tidy
```

### 2. Настрой переменные окружения

```bash
cp .env.example .env
# Открой .env и вставь свой OpenAI API Key
```

### 3. Запусти сервер

```bash
# С AI (нужен OpenAI API Key)
OPENAI_API_KEY=sk-... go run main.go

# Без AI (демо-режим, работает без ключа)
go run main.go
```

Открой в браузере: **http://localhost:8080**

---

## 📁 Структура проекта

```
├── main.go              # HTTP сервер, роутинг
├── handlers/
│   ├── chat.go          # POST /api/chat — OpenAI запросы
│   └── menu.go          # GET  /api/menu — список блюд
├── models/
│   └── types.go         # Go структуры данных
├── data/
│   └── menu.json        # Меню ресторана (редактируй здесь!)
├── static/
│   ├── index.html       # Главная страница
│   ├── style.css        # Стили
│   └── chat.js          # Чат-виджет
├── Dockerfile           # Docker образ
└── .env.example         # Пример переменных окружения
```

---

## 🌐 API Endpoints

| Метод | URL | Описание |
|-------|-----|----------|
| `GET` | `/` | Главная страница сайта |
| `GET` | `/api/menu` | Полное меню в JSON |
| `POST` | `/api/chat` | Вопрос AI-ассистенту |
| `GET` | `/health` | Проверка работы сервера |

### Пример запроса к чату:

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Что есть в меню?"}'
```

Ответ:
```json
{"reply": "В нашем меню есть донеры, гарниры, соусы и напитки! 🌯..."}
```

---

## 🍽️ Настройка меню

Отредактируй файл `data/menu.json` — добавь свои блюда, цены и описания.
Перезапусти сервер после изменений.

---

## 🐳 Docker

```bash
# Собрать образ
docker build -t donerka-house .

# Запустить
docker run -p 8080:8080 -e OPENAI_API_KEY=sk-... donerka-house
```

---

## ☁️ Деплой

### Railway (рекомендуется, бесплатный тариф)
1. Зайди на [railway.app](https://railway.app)
2. "New Project" → "Deploy from GitHub"
3. Выбери этот репозиторий
4. В переменных окружения добавь `OPENAI_API_KEY`
5. Railway автоматически определит Dockerfile и задеплоит

### Render
1. [render.com](https://render.com) → "New Web Service"
2. Подключи GitHub репо
3. Build Command: `go build -o server .`
4. Start Command: `./server`
5. Добавь env var `OPENAI_API_KEY`

---

## 🔑 Получить OpenAI API Key

1. Зарегистрируйся на [platform.openai.com](https://platform.openai.com)
2. Перейди в [API Keys](https://platform.openai.com/api-keys)
3. Нажми "Create new secret key"
4. Скопируй ключ и вставь в `.env`

> **Модель**: используется `gpt-4o-mini` (~\$0.0002 за запрос — очень дёшево)

---

## 🛠️ Технологии

- **Backend**: Go 1.21 (`net/http`)
- **AI**: OpenAI GPT-4o-mini (`go-openai`)
- **Frontend**: Vanilla HTML/CSS/JS (без фреймворков)
- **Деплой**: Docker + Railway/Render
