# ⚡ EKT.KZ — AI Ассистент

AI-консультант интернет-магазина электрооборудования **ekt.kz** на Go + OpenAI GPT-4o-mini.

## Быстрый старт

~~~bash
git clone https://github.com/BAITC-Hacks/hack-400d70df-donerka-house
cd hack-400d70df-donerka-house
go mod tidy

# полноценный диалог через OpenAI
OPENAI_API_KEY=sk-... go run main.go

# локальный demo-режим: условия, наличие загруженной детали и корзина работают без ключа
go run main.go
~~~

Открыть: http://localhost:8080

## Что реализовано

- Каталог из data/products.json и data/products2.json с дедупликацией.
- Детальная карточка через GET /api/product/{id}: наличие по складам, характеристики, ссылка на detail API и сертификаты, если они загружены.
- Аналоги по RECOMMEND, бренду, номинальному току, числу полюсов, отключающей способности и похожей серии.
- Структурированные условия оплаты, доставки, минимального заказа и возврата из data/purchase_terms.json.
- Анонимная серверная сессия с историей последних 20 сообщений.
- Безопасный двухшаговый сценарий корзины: запрос → подтверждение «Да» → повторная проверка остатка → добавление.
- Прямая ссылка на корзину /cart/{session_id} и JSON endpoint GET /api/cart/{session_id}.

Важно: предоставленный data/detail.json содержит точный остаток только для одной позиции. Для остальных карточек ассистент не выдумывает наличие и сообщает, что detail API нужно загрузить или уточнить у менеджера.

## API

| Метод | Endpoint | Описание |
|---|---|---|
| GET | / | Главная страница |
| GET | /api/catalog | Список товаров |
| GET | /api/product/{id} | Товар, detail, сертификаты |
| POST | /api/chat | Чат с message и session_id |
| GET | /api/cart/{session_id} | Содержимое корзины |
| GET | /cart/{session_id} | Страница корзины |
| GET | /health | Статус сервера |

Пример:

~~~bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"Хочу купить 5 шт. арт. 200300285_","session_id":"demo"}'

curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"Да","session_id":"demo"}'

# затем откройте http://localhost:8080/cart/demo
~~~

## Форматы данных

products.json и products2.json используют формат страниц API ekt.kz. detail.json содержит id, article, quantity, stores, properties. certificates.json и purchase_terms.json — локальные конфигурационные файлы. Демо-ссылки сертификатов помечаются is_demo: true и не выдаются за подтвержденные документы.

## Docker и деплой

~~~bash
docker build -t ekt-ai .
docker run -p 8080:8080 -e OPENAI_API_KEY=sk-... ekt-ai
~~~

Для Railway/Render подключите этот GitHub-репозиторий и задайте OPENAI_API_KEY в переменных окружения. OPENAI_MODEL необязателен и по умолчанию равен gpt-4o-mini.

## Стек

- Backend: Go 1.21, net/http
- AI: OpenAI Chat Completions
- Data: JSON-файлы каталога
- Frontend: HTML/CSS/JavaScript
- Deploy: Docker + Railway/Render
