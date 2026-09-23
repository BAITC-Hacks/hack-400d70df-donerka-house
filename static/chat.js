// ===================== CHAT.JS — EKT.KZ AI Assistant =====================

const API_CHAT = '/api/chat';
let isOpen = false;
let isBusy = false;
let chipsHidden = false;

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

      // Render product cards if returned
      if (data.products && data.products.length > 0) {
        appendProductCards(data.products);
      }

      // Handle Add to Cart action from AI
      if (data.cart_action) {
        addToCart(data.cart_action);
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
let cart = [];

function addToCart(action) {
  const existing = cart.find(i => i.article === action.article);
  if (existing) {
    existing.quantity += action.quantity;
  } else {
    cart.push(action);
  }
  updateCartUI();
  showToast(`✅ ${action.name} (x${action.quantity}) добавлен в корзину!`);
}

function updateCartUI() {
  const count = cart.reduce((acc, i) => acc + i.quantity, 0);
  const total = cart.reduce((acc, i) => acc + (i.price * i.quantity), 0);
  
  const countEl = document.getElementById('cartCount');
  const totalEl = document.getElementById('cartTotal');
  
  if (countEl) countEl.textContent = count;
  if (totalEl) totalEl.textContent = total.toLocaleString('ru-KZ') + ' тг';
  
  // Animate cart badge
  if (countEl) {
    countEl.style.transform = 'scale(1.5)';
    setTimeout(() => { countEl.style.transform = 'scale(1)'; }, 200);
  }
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

// Append a plain text message bubble
function appendMsg(text, role) {
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.className = `msg ${role}`;

  const bubble = document.createElement('div');
  bubble.className = 'msg-bubble';
  bubble.innerHTML = text.replace(/\n/g, '<br/>');

  const time = document.createElement('div');
  time.className = 'msg-time';
  time.textContent = now();

  wrap.appendChild(bubble);
  wrap.appendChild(time);
  container.appendChild(wrap);
  scrollBottom();
}

// Append product cards with image, name, price, link
function appendProductCards(products) {
  const container = document.getElementById('chatMessages');

  const wrap = document.createElement('div');
  wrap.className = 'msg bot';

  const cardsWrap = document.createElement('div');
  cardsWrap.className = 'product-cards-row';

  products.forEach(p => {
    const card = document.createElement('a');
    card.className = 'chat-product-card';
    card.href = p.url;
    card.target = '_blank';
    card.rel = 'noopener noreferrer';

    const imgWrap = document.createElement('div');
    imgWrap.className = 'chat-product-img-wrap';

    if (p.image) {
      const img = document.createElement('img');
      img.src = p.image;
      img.alt = p.name;
      img.className = 'chat-product-img';
      img.onerror = () => {
        img.style.display = 'none';
        placeholder.style.display = 'flex';
      };
      imgWrap.appendChild(img);
    }

    const placeholder = document.createElement('div');
    placeholder.className = 'chat-product-placeholder';
    placeholder.textContent = '⚡';
    placeholder.style.display = p.image ? 'none' : 'flex';
    imgWrap.appendChild(placeholder);

    const info = document.createElement('div');
    info.className = 'chat-product-info';

    const article = document.createElement('div');
    article.className = 'chat-product-article';
    article.textContent = 'Арт: ' + (p.article || '—');

    const name = document.createElement('div');
    name.className = 'chat-product-name';
    name.textContent = p.name;

    const footer = document.createElement('div');
    footer.className = 'chat-product-footer';

    const price = document.createElement('span');
    price.className = 'chat-product-price';
    price.textContent = p.price ? p.price.toLocaleString('ru-KZ') + ' тг' : 'По запросу';

    const link = document.createElement('span');
    link.className = 'chat-product-link';
    link.textContent = 'Подробнее →';

    footer.appendChild(price);
    footer.appendChild(link);

    info.appendChild(article);
    info.appendChild(name);
    info.appendChild(footer);

    card.appendChild(imgWrap);
    card.appendChild(info);
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
  wrap.innerHTML = `<div class="msg-bubble"><span class="dot"></span><span class="dot"></span><span class="dot"></span></div>`;
  container.appendChild(wrap);
  scrollBottom();
  return id;
}

function removeTyping(id) {
  const el = document.getElementById(id);
  if (el) el.remove();
}

function scrollBottom() {
  const c = document.getElementById('chatMessages');
  setTimeout(() => { c.scrollTop = c.scrollHeight; }, 60);
}

function now() {
  const d = new Date();
  return d.getHours().toString().padStart(2,'0') + ':' + d.getMinutes().toString().padStart(2,'0');
}

// Close on outside click
document.addEventListener('click', e => {
  const widget = document.getElementById('chatWidget');
  const bubble = document.getElementById('chatBubble');
  if (isOpen && !widget.contains(e.target) && !bubble.contains(e.target)) {
    toggleChat();
  }
});
