// ===================== CHAT.JS — EKT.KZ AI Assistant =====================

const API_CHAT = '/api/chat';
const API_CART = '/api/cart';
let isOpen = false;
let isBusy = false;
let chipsHidden = false;
let cart = [];

function safeURL(value) {
  try {
    const url = new URL(value || '#', window.location.origin);
    return ['http:', 'https:'].includes(url.protocol) ? url.href : '#';
  } catch {
    return '#';
  }
}

function toggleChat() {
  const widget = document.getElementById('chatWidget');
  isOpen = !isOpen;
  if (isOpen) {
    widget.classList.add('open');
    scrollBottom();
    setTimeout(() => document.getElementById('chatInput').focus(), 100);
  } else {
    widget.classList.remove('open');
  }
}

function openChat() {
  if (!isOpen) toggleChat();
}

function sendChip(btn) {
  hideChips();
  sendText(btn.textContent.trim());
}

function sendMsg() {
  const inp = document.getElementById('chatInput');
  const text = inp.value.trim();
  if (!text || isBusy) return;
  inp.value = '';
  hideChips();
  sendText(text);
}

function hideChips() {
  if (!chipsHidden) {
    document.getElementById('chatChips').style.display = 'none';
    chipsHidden = true;
  }
}

async function sendText(text) {
  if (isBusy) return;
  isBusy = true;
  document.getElementById('chatSendBtn').disabled = true;

  appendMsg(text, 'user');
  const typingId = showTyping();
  scrollBottom();

  try {
    const res = await fetch(API_CHAT, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: text }),
    });

    removeTyping(typingId);

    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      appendMsg('⚠️ ' + (err.error || 'Ошибка. Попробуйте снова.'), 'bot');
    } else {
      const data = await res.json();
      appendMsg(data.reply || '...', 'bot');

      if (data.products && data.products.length > 0) {
        appendProductCards(data.products);
      }

      if (data.cart) {
        applyCart(data.cart);
      } else if (data.cart_action) {
        // Backward-compatible fallback for an older backend response.
        await addToCart(data.cart_action, false);
      }
      if (data.cart_action) {
        showToast(`✅ ${data.cart_action.name} (x${data.cart_action.quantity}) добавлен в корзину!`);
      }
    }
  } catch {
    removeTyping(typingId);
    appendMsg('⚠️ Нет соединения с сервером.', 'bot');
  } finally {
    isBusy = false;
    document.getElementById('chatSendBtn').disabled = false;
    scrollBottom();
    document.getElementById('chatInput').focus();
  }
}

// ==== CART MANAGEMENT ====

async function loadCart() {
  try {
    const res = await fetch(API_CART);
    if (res.ok) applyCart(await res.json());
  } catch {
    showToast('⚠️ Не удалось загрузить корзину');
  }
}

function applyCart(data) {
  cart = Array.isArray(data.items) ? data.items : [];
  updateCartUI();
  renderCartPanel();
}

async function addToCart(action, notify = true) {
  try {
    const res = await fetch(API_CART, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ article: action.article, quantity: action.quantity }),
    });
    const data = await res.json();
    if (!res.ok) {
      showToast('⚠️ ' + (data.error || 'Не удалось добавить товар'));
      return false;
    }
    applyCart(data);
    if (notify) showToast(`✅ ${action.name} (x${action.quantity}) добавлен в корзину!`);
    return true;
  } catch {
    showToast('⚠️ Нет соединения с сервером');
    return false;
  }
}

async function removeCartItem(article) {
  try {
    const res = await fetch(`${API_CART}?article=${encodeURIComponent(article)}`, { method: 'DELETE' });
    if (res.ok) applyCart(await res.json());
  } catch {
    showToast('⚠️ Не удалось изменить корзину');
  }
}

async function clearCart() {
  try {
    const res = await fetch(API_CART, { method: 'DELETE' });
    if (res.ok) applyCart(await res.json());
  } catch {
    showToast('⚠️ Не удалось очистить корзину');
  }
}

function toggleCart() {
  const panel = document.getElementById('cartPanel');
  if (panel) panel.classList.toggle('open');
}

function closeCart() {
  const panel = document.getElementById('cartPanel');
  if (panel) panel.classList.remove('open');
}

function updateCartUI() {
  const count = cart.reduce((acc, item) => acc + item.quantity, 0);
  const total = cart.reduce((acc, item) => acc + (item.price * item.quantity), 0);
  const countEl = document.getElementById('cartCount');
  const totalEl = document.getElementById('cartTotal');
  const panelTotalEl = document.getElementById('cartPanelTotal');

  if (countEl) countEl.textContent = count;
  if (totalEl) totalEl.textContent = total.toLocaleString('ru-KZ') + ' тг';
  if (panelTotalEl) panelTotalEl.textContent = total.toLocaleString('ru-KZ') + ' тг';

  if (countEl) {
    countEl.style.transform = 'scale(1.5)';
    setTimeout(() => { countEl.style.transform = 'scale(1)'; }, 200);
  }
}

function renderCartPanel() {
  const container = document.getElementById('cartItems');
  const clearButton = document.getElementById('clearCartBtn');
  if (!container) return;
  container.textContent = '';
  if (clearButton) clearButton.disabled = cart.length === 0;

  if (cart.length === 0) {
    const empty = document.createElement('p');
    empty.className = 'cart-empty';
    empty.textContent = 'Корзина пока пуста';
    container.appendChild(empty);
    return;
  }

  cart.forEach(item => {
    const row = document.createElement('div');
    row.className = 'cart-item';

    const details = document.createElement('div');
    details.className = 'cart-item-details';
    const name = document.createElement('div');
    name.className = 'cart-item-name';
    name.textContent = item.name;
    const meta = document.createElement('div');
    meta.className = 'cart-item-meta';
    meta.textContent = `Арт. ${item.article} · ${item.quantity} шт.`;
    details.append(name, meta);

    const controls = document.createElement('div');
    controls.className = 'cart-item-controls';
    const price = document.createElement('span');
    price.textContent = (item.price * item.quantity).toLocaleString('ru-KZ') + ' тг';
    const remove = document.createElement('button');
    remove.className = 'cart-remove-btn';
    remove.type = 'button';
    remove.title = 'Удалить товар';
    remove.textContent = '×';
    remove.addEventListener('click', () => removeCartItem(item.article));
    controls.append(price, remove);

    row.append(details, controls);
    container.appendChild(row);
  });
}

function showToast(msg) {
  const container = document.getElementById('toastContainer');
  if (!container) return;
  const toast = document.createElement('div');
  toast.className = 'toast';
  toast.textContent = msg;
  container.appendChild(toast);
  setTimeout(() => {
    toast.classList.add('hide');
    setTimeout(() => toast.remove(), 300);
  }, 4000);
}

function appendMsg(text, role) {
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.className = `msg ${role}`;
  const bubble = document.createElement('div');
  bubble.className = 'msg-bubble';
  bubble.textContent = text;
  const time = document.createElement('div');
  time.className = 'msg-time';
  time.textContent = now();
  wrap.append(bubble, time);
  container.appendChild(wrap);
  scrollBottom();
}

function appendProductCards(products) {
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.className = 'msg bot';
  const cardsWrap = document.createElement('div');
  cardsWrap.className = 'product-cards-row';

  products.forEach(product => {
    const card = document.createElement('div');
    card.className = 'chat-product-card';

    const imgWrap = document.createElement('div');
    imgWrap.className = 'chat-product-img-wrap';
    const placeholder = document.createElement('div');
    placeholder.className = 'chat-product-placeholder';
    placeholder.textContent = '⚡';
    imgWrap.appendChild(placeholder);
    if (product.image && safeURL(product.image) !== '#') {
      const img = document.createElement('img');
      img.src = safeURL(product.image);
      img.alt = product.name || 'Товар';
      img.className = 'chat-product-img';
      img.onerror = () => { img.remove(); };
      imgWrap.insertBefore(img, placeholder);
      placeholder.style.display = 'none';
    }

    const info = document.createElement('div');
    info.className = 'chat-product-info';
    const article = document.createElement('div');
    article.className = 'chat-product-article';
    article.textContent = 'Арт: ' + (product.article || '—');
    const name = document.createElement('div');
    name.className = 'chat-product-name';
    name.textContent = product.name || 'Товар';
    const footer = document.createElement('div');
    footer.className = 'chat-product-footer';
    const price = document.createElement('span');
    price.className = 'chat-product-price';
    price.textContent = product.price ? product.price.toLocaleString('ru-KZ') + ' тг' : 'По запросу';
    const link = document.createElement('a');
    link.className = 'chat-product-link';
    link.textContent = 'Подробнее →';
    link.href = safeURL(product.url);
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    const addButton = document.createElement('button');
    addButton.type = 'button';
    addButton.className = 'chat-add-btn';
    addButton.textContent = 'В корзину';
    addButton.addEventListener('click', () => addToCart({
      article: product.article,
      name: product.name,
      quantity: 1,
    }));
    footer.append(price, link, addButton);
    info.append(article, name, footer);
    card.append(imgWrap, info);
    cardsWrap.appendChild(card);
  });

  wrap.appendChild(cardsWrap);
  container.appendChild(wrap);
  scrollBottom();
}

function showTyping() {
  const id = 'typ-' + Date.now();
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.id = id;
  wrap.className = 'msg bot typing';
  wrap.innerHTML = '<div class="msg-bubble"><span class="dot"></span><span class="dot"></span><span class="dot"></span></div>';
  container.appendChild(wrap);
  scrollBottom();
  return id;
}

function removeTyping(id) {
  const el = document.getElementById(id);
  if (el) el.remove();
}

function scrollBottom() {
  const container = document.getElementById('chatMessages');
  if (container) setTimeout(() => { container.scrollTop = container.scrollHeight; }, 60);
}

function now() {
  const date = new Date();
  return date.getHours().toString().padStart(2, '0') + ':' + date.getMinutes().toString().padStart(2, '0');
}

document.addEventListener('DOMContentLoaded', loadCart);

document.addEventListener('click', event => {
  const widget = document.getElementById('chatWidget');
  const bubble = document.getElementById('chatBubble');
  const cartPanel = document.getElementById('cartPanel');
  const cartButton = document.getElementById('cartButton');
  if (isOpen && widget && bubble && !widget.contains(event.target) && !bubble.contains(event.target)) {
    toggleChat();
  }
  if (cartPanel && cartPanel.classList.contains('open') && !cartPanel.contains(event.target) && !cartButton.contains(event.target)) {
    closeCart();
  }
});
