// ===================== CHAT.JS — EKT.KZ AI Assistant =====================

const API_CHAT = '/api/chat';
const SESSION_STORAGE_KEY = 'ekt_session_id';
let isOpen = false;
let isBusy = false;
let chipsHidden = false;

function makeSessionId() {
  if (window.crypto && typeof window.crypto.randomUUID === 'function') {
    return window.crypto.randomUUID();
  }
  return 'session-' + Date.now() + '-' + Math.random().toString(16).slice(2);
}

let sessionId = localStorage.getItem(SESSION_STORAGE_KEY);
if (!sessionId) {
  sessionId = makeSessionId();
  localStorage.setItem(SESSION_STORAGE_KEY, sessionId);
}

function updateCartLink(url) {
  const link = document.getElementById('cartLink');
  if (link) {
    link.href = url || ('/cart/' + encodeURIComponent(sessionId));
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
      body: JSON.stringify({ message: text, session_id: sessionId }),
    });

    removeTyping(typingId);

    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      appendMsg('⚠️ ' + (err.error || 'Ошибка. Попробуйте снова.'), 'bot');
    } else {
      const data = await res.json();
      if (data.session_id) {
        sessionId = data.session_id;
        localStorage.setItem(SESSION_STORAGE_KEY, sessionId);
      }
      updateCartLink(data.cart_url);
      appendMsg(data.reply || '...', 'bot');
      if (data.cart_url) appendCartLink(data.cart_url);
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

function appendMsg(text, role) {
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.className = 'msg ' + role;

  const bubble = document.createElement('div');
  bubble.className = 'msg-bubble';
  String(text ?? '').split('\n').forEach((line, index) => {
    if (index > 0) bubble.appendChild(document.createElement('br'));
    bubble.appendChild(document.createTextNode(line));
  });

  const time = document.createElement('div');
  time.className = 'msg-time';
  time.textContent = now();

  wrap.appendChild(bubble);
  wrap.appendChild(time);
  container.appendChild(wrap);
  scrollBottom();
}

function appendCartLink(url) {
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.className = 'msg bot';
  const bubble = document.createElement('div');
  bubble.className = 'msg-bubble';
  const link = document.createElement('a');
  link.href = url;
  link.textContent = '🛒 Открыть корзину';
  link.className = 'chat-cart-link';
  bubble.appendChild(link);
  wrap.appendChild(bubble);
  container.appendChild(wrap);
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
  const c = document.getElementById('chatMessages');
  setTimeout(() => { c.scrollTop = c.scrollHeight; }, 50);
}

function now() {
  const d = new Date();
  return d.getHours().toString().padStart(2, '0') + ':' + d.getMinutes().toString().padStart(2, '0');
}

document.addEventListener('DOMContentLoaded', () => {
  updateCartLink();
});

document.addEventListener('click', e => {
  const widget = document.getElementById('chatWidget');
  const bubble = document.getElementById('chatBubble');
  if (isOpen && !widget.contains(e.target) && !bubble.contains(e.target)) {
    toggleChat();
  }
});
